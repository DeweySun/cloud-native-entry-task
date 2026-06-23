package controller

import (
	"context"
	"log"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
)

const (
	group       = "dbcp.shopee.io"
	version     = "v1alpha1"
	crResource  = "dbcp-entry-services"
	finalizer   = "dbcp.shopee.io/dbcp-entry-service-finalizer"
	defaultSync = 5 * time.Second
)

var dbcpGVR = schema.GroupVersionResource{
	Group:    group,
	Version:  version,
	Resource: crResource,
}

type Options struct {
	Namespace string
	Resync    time.Duration
}

type Controller struct {
	kube      kubernetes.Interface
	dyn       dynamic.Interface
	namespace string
	resync    time.Duration
}

func New(kube kubernetes.Interface, dyn dynamic.Interface, opts Options) *Controller {
	if opts.Resync <= 0 {
		opts.Resync = defaultSync
	}
	return &Controller{
		kube:      kube,
		dyn:       dyn,
		namespace: opts.Namespace,
		resync:    opts.Resync,
	}
}

func (c *Controller) Run(ctx context.Context) error {
	ticker := time.NewTicker(c.resync)
	defer ticker.Stop()

	for {
		if err := c.reconcileAll(ctx); err != nil {
			log.Printf("reconcile all failed: %v", err)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func (c *Controller) reconcileAll(ctx context.Context) error {
	list, err := c.dbcpResources(c.namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}
	for i := range list.Items {
		cr := list.Items[i].DeepCopy()
		if err := c.reconcileCR(ctx, cr); err != nil {
			log.Printf("reconcile %s/%s failed: %v", cr.GetNamespace(), cr.GetName(), err)
		}
	}
	return nil
}

func (c *Controller) reconcileCR(ctx context.Context, cr *unstructured.Unstructured) error {
	if cr.GetDeletionTimestamp() != nil {
		if err := c.cleanupOwnedResources(ctx, cr); err != nil {
			return err
		}
		if hasFinalizer(cr, finalizer) {
			return c.removeFinalizer(ctx, cr)
		}
		return nil
	}

	if !hasFinalizer(cr, finalizer) {
		updated, err := c.addFinalizer(ctx, cr)
		if err != nil {
			return err
		}
		cr = updated
	}

	spec, err := parseSpec(cr)
	if err != nil {
		return err
	}
	specHash := desiredSpecHash(cr, spec)
	if err := c.ensureConfigMap(ctx, cr, spec); err != nil {
		return err
	}
	if err := c.ensureService(ctx, cr, spec); err != nil {
		return err
	}
	if err := c.reconcilePods(ctx, cr, spec, specHash); err != nil {
		return err
	}
	return nil
}

func (c *Controller) dbcpResources(namespace string) dynamic.ResourceInterface {
	if namespace == "" {
		namespace = metav1.NamespaceAll
	}
	return c.dyn.Resource(dbcpGVR).Namespace(namespace)
}

func (c *Controller) addFinalizer(ctx context.Context, cr *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	updated := cr.DeepCopy()
	updated.SetFinalizers(append(updated.GetFinalizers(), finalizer))
	return c.dbcpResources(cr.GetNamespace()).Update(ctx, updated, metav1.UpdateOptions{})
}

func (c *Controller) removeFinalizer(ctx context.Context, cr *unstructured.Unstructured) error {
	updated := cr.DeepCopy()
	finalizers := updated.GetFinalizers()
	out := finalizers[:0]
	for _, item := range finalizers {
		if item != finalizer {
			out = append(out, item)
		}
	}
	updated.SetFinalizers(out)
	_, err := c.dbcpResources(cr.GetNamespace()).Update(ctx, updated, metav1.UpdateOptions{})
	return err
}

func (c *Controller) cleanupOwnedResources(ctx context.Context, cr *unstructured.Unstructured) error {
	namespace := cr.GetNamespace()
	selector := managedSelector(cr.GetName())

	pods, err := c.kube.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{LabelSelector: selector})
	if err != nil {
		return err
	}
	for i := range pods.Items {
		name := pods.Items[i].Name
		if err := c.kube.CoreV1().Pods(namespace).Delete(ctx, name, metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
			return err
		}
	}

	if err := c.kube.CoreV1().Services(namespace).Delete(ctx, cr.GetName(), metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	if err := c.kube.CoreV1().ConfigMaps(namespace).Delete(ctx, configMapName(cr.GetName()), metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	return nil
}

func hasFinalizer(cr *unstructured.Unstructured, value string) bool {
	for _, item := range cr.GetFinalizers() {
		if item == value {
			return true
		}
	}
	return false
}
