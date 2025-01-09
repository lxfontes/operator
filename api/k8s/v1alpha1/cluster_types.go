/*
Copyright 2024.

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
	"go.wasmcloud.dev/operator/api/condition"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type NatsSpec struct {
	// +kubebuilder:validation:Optional
	Managed *bool `json:"managed,omitempty"`
}

type WadmSpec struct {
	// +kubebuilder:validation:Optional
	Managed *bool `json:"managed,omitempty"`
}

type PolicySpec struct {
	Rules []corev1.ObjectReference `json:"rules,omitempty"`
}

type SecretSpec struct {
	// Managed indicates whether the secret is managed by the operator.
	// A backend named "kubernetes" is managed by the operator.
	Disable bool `json:"managed,omitempty"`
}

type ClusterAddons struct {
	Policy PolicySpec `json:"policy,omitempty"`
	Secret SecretSpec `json:"secret,omitempty"`
	// Config Service?
	// Observability configuration
	// Certificates configuration?
}

// ClusterSpec defines the desired state of Cluster.
type ClusterSpec struct {
	Nats   NatsSpec      `json:"nats"`
	Wadm   WadmSpec      `json:"wadm"`
	Addons ClusterAddons `json:"addons"`
}

type NatsStatus struct {
	Managed bool `json:"managed"`
}

type WadmStatus struct {
	Managed bool `json:"managed"`
}

// ClusterStatus defines the observed state of Cluster.
type ClusterStatus struct {
	condition.ConditionedStatus `json:",inline"`
	ObservedGeneration          int64 `json:"observedGeneration,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// Cluster is the Schema for the clusters API.
// This type is not used directly and may be implemented in the future.
type Cluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ClusterSpec   `json:"spec,omitempty"`
	Status ClusterStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ClusterList contains a list of Cluster.
type ClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Cluster `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Cluster{}, &ClusterList{})
}
