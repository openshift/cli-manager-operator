package v1

import (
	operatorv1 "github.com/openshift/api/operator/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +k8s:openapi-gen=true
// +genclient
// +kubebuilder:object:root=true
// +kubebuilder:storageversion
// +kubebuilder:subresource:status

// CliManager is the Schema for the CliManager API
type CliManager struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// spec holds user settable values for configuration
	// +kubebuilder:validation:Required
	// +required
	Spec CliManagerSpec `json:"spec"`
	// status holds observed values from the cluster. They may not be overridden.
	// +kubebuilder:validation:Optional
	// +optional
	Status CliManagerStatus `json:"status"`
}

// CliManagerSpec defines the desired state of CliManager
type CliManagerSpec struct {
	operatorv1.OperatorSpec `json:",inline"`
}

// CliManagerStatus defines the observed state of CliManager
type CliManagerStatus struct {
	operatorv1.OperatorStatus `json:",inline"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
//+kubebuilder:object:root=true

// CliManagerList contains a list of CliManager
type CliManagerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []CliManager `json:"items"`
}
