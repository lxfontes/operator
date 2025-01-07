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

type ContainerSpec struct {
	Image                    string                        `json:"image,omitempty"`
	Command                  []string                      `json:"command,omitempty"`
	Args                     []string                      `json:"args,omitempty"`
	WorkingDir               string                        `json:"workingDir,omitempty"`
	Env                      []corev1.EnvVar               `json:"env,omitempty"`
	EnvFrom                  []corev1.EnvFromSource        `json:"envFrom,omitempty"`
	ImagePullSecrets         []corev1.LocalObjectReference `json:"imagePullSecrets,omitempty"`
	ImagePullPolicy          corev1.PullPolicy             `json:"imagePullPolicy,omitempty"`
	Resources                corev1.ResourceRequirements   `json:"resources,omitempty"`
	ContainerSecurityContext corev1.SecurityContext        `json:"containerSecurityContext,omitempty"`
	ReadinessProbe           *corev1.Probe                 `json:"readinessProbe,omitempty"`
	LivenessProbe            *corev1.Probe                 `json:"livenessProbe,omitempty"`
	VolumeMounts             []corev1.VolumeMount          `json:"volumeMounts,omitempty"`
}

type ReplicaSpec struct {
	Labels                       map[string]string                 `json:"labels,omitempty"`
	Affinity                     *corev1.Affinity                  `json:"affinity,omitempty"`
	AutomountServiceAccountToken *bool                             `json:"automountServiceAccountToken,omitempty"`
	NodeSelector                 map[string]string                 `json:"nodeSelector,omitempty"`
	Tolerations                  []corev1.Toleration               `json:"tolerations,omitempty"`
	TopologySpreadConstraints    []corev1.TopologySpreadConstraint `json:"topologySpreadConstraints,omitempty"`
	SecurityContext              corev1.PodSecurityContext         `json:"securityContext,omitempty"`
	InitContainers               []ContainerSpec                   `json:"initContainers,omitempty"`
	Volumes                      []corev1.Volume                   `json:"volumes,omitempty"`
}

type HostGroupSpec struct {
	ReplicaSpec   `json:",inline"`
	ContainerSpec `json:",inline"`
}

// HostGroup defines the desired state of HostGroup.
type HostGroup struct {
	// +kubebuilder:validation:Required
	Name string `json:"name,omitempty"`
	// +kubebuilder:validation:Optional
	HostLabels map[string]string `json:"hostLabels,omitempty"`

	// customizations
	Spec *HostGroupSpec `json:"spec,omitempty"`
}

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
	HostGroups []HostGroupSpec `json:"hostGroups,omitempty"`
	Nats       NatsSpec        `json:"nats"`
	Wadm       WadmSpec        `json:"wadm"`
	Addons     ClusterAddons   `json:"addons"`
}

type NatsStatus struct {
	Managed bool `json:"managed"`
}

type WadmStatus struct {
	Managed bool `json:"managed"`
}

type HostGroupStatus struct {
	Name string `json:"name,omitempty"`
}

// ClusterStatus defines the observed state of Cluster.
type ClusterStatus struct {
	condition.ConditionedStatus `json:",inline"`
	HostGroups                  []HostGroupStatus `json:"hostGroups,omitempty"`
	ObservedGeneration          int64             `json:"observedGeneration,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// Cluster is the Schema for the clusters API.
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
