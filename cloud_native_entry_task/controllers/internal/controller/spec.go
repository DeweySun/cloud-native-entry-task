package controller

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
)

type EntrySpec struct {
	Config  EntryConfig
	Service EntryService
}

type EntryConfig struct {
	TargetDB          string
	TargetRedis       string
	ServiceExportPort int32
}

type EntryService struct {
	Image     string
	Replicas  int32
	Resources corev1.ResourceRequirements
}

func parseSpec(cr *unstructured.Unstructured) (EntrySpec, error) {
	targetDB, found, err := unstructured.NestedString(cr.Object, "spec", "config", "targetDB")
	if err != nil || !found || targetDB == "" {
		return EntrySpec{}, fmt.Errorf("spec.config.targetDB is required")
	}
	targetRedis, found, err := unstructured.NestedString(cr.Object, "spec", "config", "targetRedis")
	if err != nil || !found || targetRedis == "" {
		return EntrySpec{}, fmt.Errorf("spec.config.targetRedis is required")
	}
	port, found, err := unstructured.NestedInt64(cr.Object, "spec", "config", "serviceExportPort")
	if err != nil || !found || port < 1 || port > 65535 {
		return EntrySpec{}, fmt.Errorf("spec.config.serviceExportPort must be between 1 and 65535")
	}
	image, found, err := unstructured.NestedString(cr.Object, "spec", "service", "image")
	if err != nil || !found || image == "" {
		return EntrySpec{}, fmt.Errorf("spec.service.image is required")
	}
	replicas, found, err := unstructured.NestedInt64(cr.Object, "spec", "service", "replicas")
	if err != nil || !found || replicas < 1 {
		return EntrySpec{}, fmt.Errorf("spec.service.replicas must be greater than zero")
	}
	resources, err := parseResources(cr)
	if err != nil {
		return EntrySpec{}, err
	}
	return EntrySpec{
		Config: EntryConfig{
			TargetDB:          targetDB,
			TargetRedis:       targetRedis,
			ServiceExportPort: int32(port),
		},
		Service: EntryService{
			Image:     image,
			Replicas:  int32(replicas),
			Resources: resources,
		},
	}, nil
}

func parseResources(cr *unstructured.Unstructured) (corev1.ResourceRequirements, error) {
	requests, err := parseResourceList(cr, "requests")
	if err != nil {
		return corev1.ResourceRequirements{}, err
	}
	limits, err := parseResourceList(cr, "limits")
	if err != nil {
		return corev1.ResourceRequirements{}, err
	}
	return corev1.ResourceRequirements{Requests: requests, Limits: limits}, nil
}

func parseResourceList(cr *unstructured.Unstructured, field string) (corev1.ResourceList, error) {
	values, found, err := unstructured.NestedStringMap(cr.Object, "spec", "service", "resources", field)
	if err != nil || !found {
		return nil, err
	}
	out := corev1.ResourceList{}
	for name, value := range values {
		quantity, err := resource.ParseQuantity(value)
		if err != nil {
			return nil, fmt.Errorf("spec.service.resources.%s.%s: %w", field, name, err)
		}
		out[corev1.ResourceName(name)] = quantity
	}
	return out, nil
}

func desiredSpecHash(cr *unstructured.Unstructured, spec EntrySpec) string {
	input := struct {
		ConfigMap map[string]string           `json:"configMap"`
		Image     string                      `json:"image"`
		Resources corev1.ResourceRequirements `json:"resources"`
		Owner     map[string]string           `json:"owner"`
		Ports     map[string]int32            `json:"ports"`
	}{
		ConfigMap: desiredConfigData(cr, spec),
		Image:     spec.Service.Image,
		Resources: spec.Service.Resources,
		Owner: map[string]string{
			"apiVersion": dbcpAPIVersion(),
			"kind":       dbcpKind(),
		},
		Ports: map[string]int32{
			"http":        appHTTPPort,
			"httpGateway": appGatewayPort,
			"tcpBackend":  appTCPPort,
			"service":     spec.Config.ServiceExportPort,
		},
	}
	data, _ := json.Marshal(input)
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])[:12]
}

func desiredConfigData(cr *unstructured.Unstructured, spec EntrySpec) map[string]string {
	return map[string]string{
		"DBCP_TARGET_DB":                  spec.Config.TargetDB,
		"DBCP_TARGET_REDIS":               spec.Config.TargetRedis,
		"DBCP_SERVICE_EXPORT_PORT":        strconv.Itoa(int(spec.Config.ServiceExportPort)),
		"APP_TCP_ADDR":                    "127.0.0.1:9000",
		"APP_HTTP_ADDR":                   "127.0.0.1:8081",
		"APP_HTTP_TCP_ADDR":               "127.0.0.1:9000",
		"APP_PROFILE_PICTURE_DIR":         "/app/runtime/profile-pictures",
		"APP_PROFILE_PICTURE_BASE_URL":    "/api/me/profile-picture",
		"APP_REDIS_KEY_PREFIX":            "go-entry-task",
		"APP_TOKEN_SECRET":                stableTokenSecret(cr),
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

func stableTokenSecret(cr *unstructured.Unstructured) string {
	seed := fmt.Sprintf("%s/%s/%s", cr.GetNamespace(), cr.GetName(), string(cr.GetUID()))
	sum := sha256.Sum256([]byte(seed))
	return hex.EncodeToString(sum[:])
}

func ownerUID(cr *unstructured.Unstructured) types.UID {
	return cr.GetUID()
}
