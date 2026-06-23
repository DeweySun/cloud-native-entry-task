package controller

import (
	"context"
	"reflect"
	"sort"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/intstr"
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

	appName       = "dbcp-entry-service"
	controllerID  = "dbcp-entry-controller"
	containerName = "dbcp-entry-service"
)

func (c *Controller) ensureConfigMap(ctx context.Context, cr *unstructured.Unstructured, spec EntrySpec) error {
	namespace := cr.GetNamespace()
	name := configMapName(cr.GetName())
	desiredData := desiredConfigData(cr, spec)
	desiredLabels := managedLabels(cr.GetName())
	desiredOwnerRefs := ownerRefs(cr)

	existing, err := c.kube.CoreV1().ConfigMaps(namespace).Get(ctx, name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		_, err = c.kube.CoreV1().ConfigMaps(namespace).Create(ctx, &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:            name,
				Namespace:       namespace,
				Labels:          desiredLabels,
				OwnerReferences: desiredOwnerRefs,
			},
			Data: desiredData,
		}, metav1.CreateOptions{})
		return err
	}
	if err != nil {
		return err
	}
	if reflect.DeepEqual(existing.Data, desiredData) &&
		reflect.DeepEqual(existing.Labels, desiredLabels) &&
		reflect.DeepEqual(existing.OwnerReferences, desiredOwnerRefs) {
		return nil
	}
	updated := existing.DeepCopy()
	updated.Labels = desiredLabels
	updated.OwnerReferences = desiredOwnerRefs
	updated.Data = desiredData
	_, err = c.kube.CoreV1().ConfigMaps(namespace).Update(ctx, updated, metav1.UpdateOptions{})
	return err
}

func (c *Controller) ensureService(ctx context.Context, cr *unstructured.Unstructured, spec EntrySpec) error {
	namespace := cr.GetNamespace()
	desiredLabels := managedLabels(cr.GetName())
	desiredOwnerRefs := ownerRefs(cr)
	desiredSelector := managedLabels(cr.GetName())
	desiredPorts := []corev1.ServicePort{{
		Name:       "http",
		Protocol:   corev1.ProtocolTCP,
		Port:       spec.Config.ServiceExportPort,
		TargetPort: intstr.FromString("http"),
	}}

	existing, err := c.kube.CoreV1().Services(namespace).Get(ctx, cr.GetName(), metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		_, err = c.kube.CoreV1().Services(namespace).Create(ctx, &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:            cr.GetName(),
				Namespace:       namespace,
				Labels:          desiredLabels,
				OwnerReferences: desiredOwnerRefs,
			},
			Spec: corev1.ServiceSpec{
				Type:     corev1.ServiceTypeClusterIP,
				Selector: desiredSelector,
				Ports:    desiredPorts,
			},
		}, metav1.CreateOptions{})
		return err
	}
	if err != nil {
		return err
	}

	updated := existing.DeepCopy()
	updated.Labels = desiredLabels
	updated.OwnerReferences = desiredOwnerRefs
	updated.Spec.Selector = desiredSelector
	updated.Spec.Ports = desiredPorts
	updated.Spec.Type = corev1.ServiceTypeClusterIP
	if reflect.DeepEqual(existing.Labels, updated.Labels) &&
		reflect.DeepEqual(existing.OwnerReferences, updated.OwnerReferences) &&
		reflect.DeepEqual(existing.Spec.Selector, updated.Spec.Selector) &&
		reflect.DeepEqual(existing.Spec.Ports, updated.Spec.Ports) &&
		existing.Spec.Type == updated.Spec.Type {
		return nil
	}
	_, err = c.kube.CoreV1().Services(namespace).Update(ctx, updated, metav1.UpdateOptions{})
	return err
}

func (c *Controller) reconcilePods(ctx context.Context, cr *unstructured.Unstructured, spec EntrySpec, specHash string) error {
	pods, err := c.kube.CoreV1().Pods(cr.GetNamespace()).List(ctx, metav1.ListOptions{LabelSelector: managedSelector(cr.GetName())})
	if err != nil {
		return err
	}

	current := make([]corev1.Pod, 0, len(pods.Items))
	for i := range pods.Items {
		pod := pods.Items[i]
		if pod.DeletionTimestamp != nil {
			continue
		}
		if pod.Status.Phase == corev1.PodFailed || pod.Status.Phase == corev1.PodSucceeded {
			if err := c.kube.CoreV1().Pods(cr.GetNamespace()).Delete(ctx, pod.Name, metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
				return err
			}
			continue
		}
		if pod.Labels[labelSpecHash] != specHash {
			if err := c.kube.CoreV1().Pods(cr.GetNamespace()).Delete(ctx, pod.Name, metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
				return err
			}
			continue
		}
		current = append(current, pod)
	}

	sort.Slice(current, func(i, j int) bool {
		return current[i].CreationTimestamp.Before(&current[j].CreationTimestamp)
	})
	for len(current) > int(spec.Service.Replicas) {
		idx := len(current) - 1
		name := current[idx].Name
		if err := c.kube.CoreV1().Pods(cr.GetNamespace()).Delete(ctx, name, metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
			return err
		}
		current = current[:idx]
	}
	for len(current) < int(spec.Service.Replicas) {
		pod := desiredPod(cr, spec, specHash)
		created, err := c.kube.CoreV1().Pods(cr.GetNamespace()).Create(ctx, pod, metav1.CreateOptions{})
		if err != nil {
			return err
		}
		current = append(current, *created)
	}
	return nil
}

func desiredPod(cr *unstructured.Unstructured, spec EntrySpec, specHash string) *corev1.Pod {
	labels := managedLabels(cr.GetName())
	labels[labelSpecHash] = specHash
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName:    cr.GetName() + "-",
			Namespace:       cr.GetNamespace(),
			Labels:          labels,
			Annotations:     map[string]string{labelSpecHash: specHash},
			OwnerReferences: ownerRefs(cr),
		},
		Spec: corev1.PodSpec{
			RestartPolicy: corev1.RestartPolicyAlways,
			Containers: []corev1.Container{{
				Name:            containerName,
				Image:           spec.Service.Image,
				ImagePullPolicy: corev1.PullIfNotPresent,
				Ports: []corev1.ContainerPort{
					{Name: "http", ContainerPort: appHTTPPort, Protocol: corev1.ProtocolTCP},
					{Name: "gateway", ContainerPort: appGatewayPort, Protocol: corev1.ProtocolTCP},
					{Name: "tcp-backend", ContainerPort: appTCPPort, Protocol: corev1.ProtocolTCP},
				},
				EnvFrom: []corev1.EnvFromSource{{
					ConfigMapRef: &corev1.ConfigMapEnvSource{
						LocalObjectReference: corev1.LocalObjectReference{Name: configMapName(cr.GetName())},
					},
				}},
				ReadinessProbe: httpProbe("/api/health", 5, 5),
				LivenessProbe:  httpProbe("/api/health", 15, 10),
				Resources:      spec.Service.Resources,
			}},
		},
	}
}

func httpProbe(path string, initialDelay, period int32) *corev1.Probe {
	return &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			HTTPGet: &corev1.HTTPGetAction{
				Path: path,
				Port: intstr.FromString("http"),
			},
		},
		InitialDelaySeconds: initialDelay,
		PeriodSeconds:       period,
	}
}

func managedLabels(crName string) map[string]string {
	return map[string]string{
		labelName:      appName,
		labelInstance:  crName,
		labelManagedBy: controllerID,
		labelCRName:    crName,
	}
}

func managedSelector(crName string) string {
	return labelManagedBy + "=" + controllerID + "," + labelCRName + "=" + crName
}

func configMapName(crName string) string {
	return crName + "-config"
}

func dbcpAPIVersion() string {
	return group + "/" + version
}

func dbcpKind() string {
	return "DbcpEntryService"
}

func ownerRefs(cr *unstructured.Unstructured) []metav1.OwnerReference {
	controller := true
	return []metav1.OwnerReference{{
		APIVersion: dbcpAPIVersion(),
		Kind:       dbcpKind(),
		Name:       cr.GetName(),
		UID:        ownerUID(cr),
		Controller: &controller,
	}}
}
