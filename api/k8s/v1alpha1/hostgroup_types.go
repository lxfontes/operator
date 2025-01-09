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

// HostGroupSpec defines the desired state of HostGroup.
type HostGroupSpec struct {
	ReplicaSpec   `json:",inline"`
	ContainerSpec `json:",inline"`
	// +kubebuilder:validation:Optional
	HostLabels map[string]string `json:"hostLabels,omitempty"`
}

// HostGroupStatus defines the observed state of HostGroup.
type HostGroupStatus struct {
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// HostGroup is the Schema for the hostgroups API.
type HostGroup struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   HostGroupSpec   `json:"spec,omitempty"`
	Status HostGroupStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// HostGroupList contains a list of HostGroup.
type HostGroupList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []HostGroup `json:"items"`
}

func init() {
	SchemeBuilder.Register(&HostGroup{}, &HostGroupList{})
}
