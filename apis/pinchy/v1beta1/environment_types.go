package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type EnvironmentPhase string

const (
	EnvironmentPhasePending EnvironmentPhase = "Pending"
	EnvironmentPhaseRunning EnvironmentPhase = "Running"
	EnvironmentPhaseFailed  EnvironmentPhase = "Failed"
)

// EnvironmentSpec defines the desired state of Environment
type EnvironmentSpec struct {
	Path string `json:"path,omitempty"`
}

// EnvironmentStatus defines the observed state of Environment
type EnvironmentStatus struct {
	Phase EnvironmentPhase `json:"phase,omitempty"`
	// PodIP is the IP address of the environment Pod, once it has been assigned.
	PodIP string `json:"podIP,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +genclient

// Environment is the Schema for the Environments API
type Environment struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   EnvironmentSpec   `json:"spec,omitempty"`
	Status EnvironmentStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// EnvironmentList contains a list of Environment
type EnvironmentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Environment `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Environment{}, &EnvironmentList{})
}
