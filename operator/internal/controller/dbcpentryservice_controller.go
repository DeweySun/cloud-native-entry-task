/*
Copyright 2026.
...
*/

package controller

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/source"

	dbcpv1alpha1 "github.com/DeweySun/cloud-native-entry-task/operator/api/v1alpha1"
)

const (
	appHTTPPort    int32 = 8080
	appGatewayPort int32 = 8081
	appTCPPort     int32 = 9000

	labelName      = "app.kubernetes.io/name"
	labelInstance  = "app.kubernetes.io/instance"
	labelManagedBy = "app.kubernetes.io/managed-by"
	labelCRName    = "dbcp.shopee.io/cr-name"
	labelSpecHash  = "dbcp.shopee.io/spec-hash"

	appName      = "dbcp-entry-service"
	controllerID = "dbcp-entry-controller"
	finalizer    = "dbcp.shopee.io/dbcp-entry-service-finalizer"

	defaultSyncInterval = 5 * time.Second
)

// DbcpEntryServiceReconciler reconciles a DbcpEntryService object
type DbcpEntryServiceReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=dbcp.shopee.io,resources=dbcpentryservices,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=dbcp.shopee.io,resources=dbcpentryservices/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=dbcp.shopee.io,resources=dbcpentryservices/finalizers,verbs=update
// +kubebuilder:rbac:groups=core,resources=pods;services;configmaps,verbs=get;list;watch;create;update;patch;delete

func (r *DbcpEntryServiceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)
	log.Info("Reconciling DbcpEntryService")

	var entry dbcpv1alpha1.DbcpEntryService
	if err := r.Get(ctx, req.NamespacedName, &entry); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Handle deletion
	if !entry.DeletionTimestamp.IsZero() {
		if err := r.cleanupOwnedResources(ctx, &entry); err != nil {
			return ctrl.Result{}, err
		}
		if controllerutil.ContainsFinalizer(&entry, finalizer) {
			controllerutil.RemoveFinalizer(&entry, finalizer)
			if err := r.Update(ctx, &entry); err != nil {
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	// Ensure finalizer is present
	if !controllerutil.ContainsFinalizer(&entry, finalizer) {
		controllerutil.AddFinalizer(&entry, finalizer)
		if err := r.Update(ctx, &entry); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	// Calculate spec hash for rollout tracking
	specHash, err := r.calculateSpecHash(&entry)
	if err != nil {
		log.Error(err, "failed to calculate spec hash")
		return ctrl.Result{}, err
	}

	// Ensure ConfigMap (specHash no longer passed because it's not used)
	if err := r.ensureConfigMap(ctx, &entry); err != nil {
		return ctrl.Result{}, err
	}

	// Ensure Service
	if err := r.ensureService(ctx, &entry); err != nil {
		return ctrl.Result{}, err
	}

	// Ensure Pods
	if err := r.reconcilePods(ctx, &entry, specHash); err != nil {
		return ctrl.Result{}, err
	}

	log.Info("Reconcile completed successfully")
	return ctrl.Result{RequeueAfter: defaultSyncInterval}, nil
}

// SetupWithManager sets up the controller with the Manager and adds a periodic full‑sync goroutine.
func (r *DbcpEntryServiceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	c, err := ctrl.NewControllerManagedBy(mgr).
		For(&dbcpv1alpha1.DbcpEntryService{}).
		Named("dbcpentryservice").
		Build(r)
	if err != nil {
		return err
	}

	// Periodic enqueue of all DbcpEntryService objects to achieve full‑sync every 5 seconds.
	events := make(chan event.GenericEvent)
	go func() {
		ticker := time.NewTicker(defaultSyncInterval)
		defer ticker.Stop()
		for range ticker.C {
			list := &dbcpv1alpha1.DbcpEntryServiceList{}
			if err := mgr.GetClient().List(context.Background(), list); err != nil {
				continue
			}
			for _, item := range list.Items {
				events <- event.GenericEvent{
					Object: item.DeepCopy(),
				}
			}
		}
	}()

	return c.Watch(source.Channel(events, &handler.EnqueueRequestForObject{}))
}

// ---------- resource helpers ----------

func (r *DbcpEntryServiceReconciler) cleanupOwnedResources(ctx context.Context, entry *dbcpv1alpha1.DbcpEntryService) error {
	log := logf.FromContext(ctx)
	namespace := entry.Namespace

	// Delete Pods
	podList := &corev1.PodList{}
	if err := r.List(ctx, podList, client.InNamespace(namespace), client.MatchingLabelsSelector{
		Selector: labels.SelectorFromSet(labels.Set{
			labelManagedBy: controllerID,
			labelCRName:    entry.Name,
		}),
	}); err != nil {
		return err
	}
	for _, pod := range podList.Items {
		if err := r.Delete(ctx, &pod); err != nil && !apierrors.IsNotFound(err) {
			return err
		}
	}

	// Delete Service
	svc := &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: entry.Name, Namespace: namespace}}
	if err := r.Delete(ctx, svc); err != nil && !apierrors.IsNotFound(err) {
		return err
	}

	// Delete ConfigMap
	cm := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: configMapName(entry.Name), Namespace: namespace}}
	if err := r.Delete(ctx, cm); err != nil && !apierrors.IsNotFound(err) {
		return err
	}

	log.Info("Cleaned up owned resources")
	return nil
}

func (r *DbcpEntryServiceReconciler) ensureConfigMap(ctx context.Context, entry *dbcpv1alpha1.DbcpEntryService) error {
	desired := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      configMapName(entry.Name),
			Namespace: entry.Namespace,
		},
	}
	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, desired, func() error {
		desired.Labels = managedLabels(entry.Name)
		desired.Data = desiredConfigData(entry)
		return controllerutil.SetControllerReference(entry, desired, r.Scheme)
	})
	return err
}

func (r *DbcpEntryServiceReconciler) ensureService(ctx context.Context, entry *dbcpv1alpha1.DbcpEntryService) error {
	desired := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      entry.Name,
			Namespace: entry.Namespace,
		},
	}
	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, desired, func() error {
		desired.Labels = managedLabels(entry.Name)
		desired.Spec = corev1.ServiceSpec{
			Type:     corev1.ServiceTypeClusterIP,
			Selector: managedLabels(entry.Name),
			Ports: []corev1.ServicePort{{
				Name:       "http",
				Protocol:   corev1.ProtocolTCP,
				Port:       entry.Spec.Config.ServiceExportPort,
				TargetPort: intstr.FromString("http"),
			}},
		}
		return controllerutil.SetControllerReference(entry, desired, r.Scheme)
	})
	return err
}

func (r *DbcpEntryServiceReconciler) reconcilePods(ctx context.Context, entry *dbcpv1alpha1.DbcpEntryService, specHash string) error {
	log := logf.FromContext(ctx)
	podList := &corev1.PodList{}
	if err := r.List(ctx, podList,
		client.InNamespace(entry.Namespace),
		client.MatchingLabelsSelector{Selector: labels.SelectorFromSet(labels.Set{
			labelManagedBy: controllerID,
			labelCRName:    entry.Name,
		})},
	); err != nil {
		return err
	}

	var current []corev1.Pod
	for _, pod := range podList.Items {
		if pod.DeletionTimestamp != nil {
			continue
		}
		if pod.Status.Phase == corev1.PodFailed || pod.Status.Phase == corev1.PodSucceeded {
			log.Info("Deleting terminated pod", "pod", pod.Name)
			if err := r.Delete(ctx, &pod); err != nil && !apierrors.IsNotFound(err) {
				return err
			}
			continue
		}
		// Delete pods whose specHash doesn't match the desired one
		if pod.Labels[labelSpecHash] != specHash {
			log.Info("Deleting outdated pod", "pod", pod.Name, "oldHash", pod.Labels[labelSpecHash], "newHash", specHash)
			if err := r.Delete(ctx, &pod); err != nil && !apierrors.IsNotFound(err) {
				return err
			}
			continue
		}
		current = append(current, pod)
	}

	// Sort by creation timestamp, oldest first
	sort.Slice(current, func(i, j int) bool {
		return current[i].CreationTimestamp.Before(&current[j].CreationTimestamp)
	})

	desiredReplicas := int(entry.Spec.Service.Replicas)

	// Scale down: remove newest pods first
	for len(current) > desiredReplicas {
		idx := len(current) - 1
		pod := current[idx]
		log.Info("Scaling down", "pod", pod.Name)
		if err := r.Delete(ctx, &pod); err != nil && !apierrors.IsNotFound(err) {
			return err
		}
		current = current[:idx]
	}

	// Scale up: create new pods
	for len(current) < desiredReplicas {
		pod := r.desiredPod(entry, specHash)
		if err := r.Create(ctx, pod); err != nil {
			return err
		}
		current = append(current, *pod)
		log.Info("Created new pod", "pod", pod.Name)
	}

	return nil
}

func (r *DbcpEntryServiceReconciler) desiredPod(entry *dbcpv1alpha1.DbcpEntryService, specHash string) *corev1.Pod {
	labels := managedLabels(entry.Name)
	labels[labelSpecHash] = specHash

	readinessProbe := &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			HTTPGet: &corev1.HTTPGetAction{
				Path: "/api/health",
				Port: intstr.FromString("http"),
			},
		},
		InitialDelaySeconds: 5,
		PeriodSeconds:       5,
	}
	livenessProbe := &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			HTTPGet: &corev1.HTTPGetAction{
				Path: "/api/health",
				Port: intstr.FromString("http"),
			},
		},
		InitialDelaySeconds: 15,
		PeriodSeconds:       10,
	}

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: entry.Name + "-",
			Namespace:    entry.Namespace,
			Labels:       labels,
			Annotations:  map[string]string{labelSpecHash: specHash},
		},
		Spec: corev1.PodSpec{
			RestartPolicy: corev1.RestartPolicyAlways,
			Containers: []corev1.Container{{
				Name:            "dbcp-entry-service",
				Image:           entry.Spec.Service.Image,
				ImagePullPolicy: corev1.PullIfNotPresent,
				Ports: []corev1.ContainerPort{
					{Name: "http", ContainerPort: appHTTPPort, Protocol: corev1.ProtocolTCP},
					{Name: "gateway", ContainerPort: appGatewayPort, Protocol: corev1.ProtocolTCP},
					{Name: "tcp-backend", ContainerPort: appTCPPort, Protocol: corev1.ProtocolTCP},
				},
				EnvFrom: []corev1.EnvFromSource{{
					ConfigMapRef: &corev1.ConfigMapEnvSource{
						LocalObjectReference: corev1.LocalObjectReference{Name: configMapName(entry.Name)},
					},
				}},
				ReadinessProbe: readinessProbe,
				LivenessProbe:  livenessProbe,
				Resources:      entry.Spec.Service.Resources,
			}},
		},
	}
	// Set owner reference using controllerutil; ignore error because the pod may be created before the owner.
	if err := controllerutil.SetControllerReference(entry, pod, r.Scheme); err != nil {
		logf.Log.Error(err, "unable to set owner reference on pod")
	}
	return pod
}

// ---------- data / hash helpers ----------

func desiredConfigData(entry *dbcpv1alpha1.DbcpEntryService) map[string]string {
	return map[string]string{
		"DBCP_TARGET_DB":                  entry.Spec.Config.TargetDB,
		"DBCP_TARGET_REDIS":               entry.Spec.Config.TargetRedis,
		"DBCP_SERVICE_EXPORT_PORT":        strconv.Itoa(int(entry.Spec.Config.ServiceExportPort)),
		"APP_TCP_ADDR":                    "127.0.0.1:9000",
		"APP_HTTP_ADDR":                   "127.0.0.1:8081",
		"APP_HTTP_TCP_ADDR":               "127.0.0.1:9000",
		"APP_PROFILE_PICTURE_DIR":         "/app/runtime/profile-pictures",
		"APP_PROFILE_PICTURE_BASE_URL":    "/api/me/profile-picture",
		"APP_REDIS_KEY_PREFIX":            "go-entry-task",
		"APP_TOKEN_SECRET":                stableTokenSecret(entry),
		"APP_DB_MAX_OPEN_CONNS":           "128",
		"APP_DB_MAX_IDLE_CONNS":           "32",
		"APP_TCP_WORKERS":                 "64",
		"APP_TCP_QUEUE_SIZE":              "2048",
		"APP_TCP_MAX_FRAME_BYTES":         "8388608",
		"APP_HTTP_MAX_BODY_BYTES":         "4194304",
		"APP_UPLOAD_MAX_BYTES":            "2097152",
		"APP_SESSION_TTL":                 "24h",
		"APP_DB_CONN_MAX_LIFETIME":        "5m",
		"APP_REDIS_DIAL_TIMEOUT":          "2s",
		"APP_REDIS_IO_TIMEOUT":            "2s",
		"APP_PROFILE_PICTURE_CACHE_SCOPE": "redis",
	}
}

func stableTokenSecret(entry *dbcpv1alpha1.DbcpEntryService) string {
	seed := fmt.Sprintf("%s/%s/%s", entry.Namespace, entry.Name, entry.GetUID())
	sum := sha256.Sum256([]byte(seed))
	return hex.EncodeToString(sum[:])
}

func (r *DbcpEntryServiceReconciler) calculateSpecHash(entry *dbcpv1alpha1.DbcpEntryService) (string, error) {
	data := map[string]interface{}{
		"configMap": desiredConfigData(entry),
		"image":     entry.Spec.Service.Image,
		"resources": entry.Spec.Service.Resources,
		"owner": map[string]string{
			"apiVersion": dbcpv1alpha1.GroupVersion.String(),
			"kind":       "DbcpEntryService",
		},
		"ports": map[string]int32{
			"http":        appHTTPPort,
			"httpGateway": appGatewayPort,
			"tcpBackend":  appTCPPort,
			"service":     entry.Spec.Config.ServiceExportPort,
		},
	}
	bytes, err := json.Marshal(data)
	if err != nil {
		return "", fmt.Errorf("failed to marshal spec hash data: %w", err)
	}
	sum := sha256.Sum256(bytes)
	return hex.EncodeToString(sum[:])[:12], nil
}

// ---------- label helpers ----------

func managedLabels(crName string) map[string]string {
	return map[string]string{
		labelName:      appName,
		labelInstance:  crName,
		labelManagedBy: controllerID,
		labelCRName:    crName,
	}
}

func configMapName(crName string) string {
	return crName + "-config"
}
