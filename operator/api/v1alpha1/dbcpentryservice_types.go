/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// DbcpEntryServiceSpec defines the desired state of DbcpEntryService.
type DbcpEntryServiceSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// +kubebuilder:validation:Required
	Config EntryConfig `json:"config"`

	// +kubebuilder:validation:Required
	Service EntryService `json:"service"`
}

// EntryConfig holds the configuration for external dependencies and export port.
type EntryConfig struct {
	// TargetDB is the MySQL DSN used by the application.
	// +kubebuilder:validation:Required
	TargetDB string `json:"targetDB"`

	// TargetRedis is the address of the Redis server (host:port).
	// +kubebuilder:validation:Required
	TargetRedis string `json:"targetRedis"`

	// ServiceExportPort is the port on which the Kubernetes Service will be exposed.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	ServiceExportPort int32 `json:"serviceExportPort"`
}

// EntryService defines the runtime aspects of the application Pods.
type EntryService struct {
	// Image is the container image for the application.
	// +kubebuilder:validation:Required
	Image string `json:"image"`

	// Replicas is the number of desired Pod replicas.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Minimum=1
	Replicas int32 `json:"replicas"`

	// Resources specifies the resource requests and limits for the container.
	// +optional
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`
}

// DbcpEntryServiceStatus defines the observed state of DbcpEntryService.
type DbcpEntryServiceStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// DbcpEntryService is the Schema for the dbcpentryservices API.
type DbcpEntryService struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DbcpEntryServiceSpec   `json:"spec,omitempty"`
	Status DbcpEntryServiceStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// DbcpEntryServiceList contains a list of DbcpEntryService.
type DbcpEntryServiceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DbcpEntryService `json:"items"`
}

func init() {
	SchemeBuilder.Register(&DbcpEntryService{}, &DbcpEntryServiceList{})
}
