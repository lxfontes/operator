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

package v1beta1

import (
	"go.wasmcloud.dev/operator/api/condition"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
)

// ApplicationPhase is a label for the condition of an application at the current time
type ApplicationPhase string

type ApplicationTrait struct {
	Type string `json:"type"`
	// +kubebuilder:pruning:PreserveUnknownFields
	Properties *runtime.RawExtension `json:"properties,omitempty"`
}

type ApplicationComponent struct {
	Name string `json:"name"`
	Type string `json:"type"`
	// ExternalRevision specified the component revisionName
	ExternalRevision string `json:"externalRevision,omitempty"`
	// +kubebuilder:pruning:PreserveUnknownFields
	Properties *runtime.RawExtension `json:"properties,omitempty"`

	DependsOn []string `json:"dependsOn,omitempty"`

	// Traits define the trait of one component, the type must be array to keep the order.
	Traits []ApplicationTrait `json:"traits,omitempty"`
}

type ApplicationPolicy struct {
	// Name is the unique name of the policy.
	// +optional
	Name string `json:"name,omitempty"`
	// Type is the type of the policy
	Type string `json:"type"`
	// +kubebuilder:pruning:PreserveUnknownFields
	Properties *runtime.RawExtension `json:"properties,omitempty"`
}

// ApplicationSpec defines the desired state of Application.
type ApplicationSpec struct {
	Components []ApplicationComponent `json:"components"`
	Policies   []ApplicationPolicy    `json:"policies,omitempty"`
}

type ApplicationTraitStatus struct {
	Type    string `json:"type"`
	Healthy bool   `json:"healthy"`
	Message string `json:"message,omitempty"`
}

type ApplicationComponentStatus struct {
	Name      string                   `json:"name"`
	Namespace string                   `json:"namespace,omitempty"`
	Cluster   string                   `json:"cluster,omitempty"`
	Env       string                   `json:"env,omitempty"`
	Healthy   bool                     `json:"healthy"`
	Message   string                   `json:"message,omitempty"`
	Traits    []ApplicationTraitStatus `json:"traits,omitempty"`
}

type Revision struct {
	Name     string `json:"name"`
	Revision int64  `json:"revision"`

	// RevisionHash record the hash value of the spec of ApplicationRevision object.
	RevisionHash string `json:"revisionHash,omitempty"`
}

type ScalerStatus struct {
	Id      string `json:"id"`
	Kind    string `json:"kind"`
	Name    string `json:"name"`
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

// ApplicationStatus defines the observed state of Application.
type ApplicationStatus struct {
	condition.ConditionedStatus `json:",inline"`

	// The generation observed by the application controller.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// The version observed by wadm.
	// +optional
	ObservedVersion string `json:"observedVersion,omitempty"`

	// The wadm status.
	// +optional
	Phase ApplicationPhase `json:"phase,omitempty"`

	// Status for each wadm scaler.
	// +optional
	ScalerStatus []ScalerStatus `json:"scalerStatus,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:categories={oam},shortName={app}
// +kubebuilder:printcolumn:name="COMPONENT",type=string,JSONPath=`.spec.components[*].name`
// +kubebuilder:printcolumn:name="TYPE",type=string,JSONPath=`.spec.components[*].type`
// +kubebuilder:printcolumn:name="PHASE",type=string,JSONPath=`.status.status`
// +kubebuilder:printcolumn:name="HEALTHY",type=boolean,JSONPath=`.status.services[*].healthy`
// +kubebuilder:printcolumn:name="STATUS",type=string,JSONPath=`.status.services[*].message`
// +kubebuilder:printcolumn:name="AGE",type=date,JSONPath=".metadata.creationTimestamp"
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Application is the Schema for the applications API.
type Application struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ApplicationSpec   `json:"spec,omitempty"`
	Status ApplicationStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ApplicationList contains a list of Application.
type ApplicationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Application `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Application{}, &ApplicationList{})
}
