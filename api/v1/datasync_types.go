package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// VMS represents a single virtual machine to be synced.
type VMS struct {
	Name       string `json:"name"`
	URL        string `json:"url"`
	SourceType string `json:"sourceType"`
}

// DataSyncSpec defines the desired state of DataSync.
type DataSyncSpec struct {
	// The unique identifier for the workspace to be synced.
	// +kubebuilder:validation:MinLength=1
	WorkspaceID string `json:"workspaceId"`

	// VMS is a list of virtual machines to be synced.
	// Each VM is identified by a name, URL, and sourceType.
	// +kubebuilder:validation:MinItems=0
	Vms []VMS `json:"vms"`
}

// DataSyncStatus defines the observed state of DataSync.
type DataSyncStatus struct {
	// +kubebuilder:validation:Enum=New;Queued;Sycning;Completed;Failed
	Phase string `json:"phase"`

	// A human-readable message providing more details about the current phase.
	Message string `json:"message,omitempty"`

	// Conditions of the DataSync resource.
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}


// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=datasyncs,scope=Namespaced,shortName=ds,singular=datasync
// +kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase",description="The current phase of the DataSync."
// +kubebuilder:printcolumn:name="WorkspaceID",type="string",JSONPath=".spec.workspaceId",description="The ID of the workspace being synced."
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// DataSync is the Schema for the datasyncs API.
type DataSync struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DataSyncSpec   `json:"spec,omitempty"`
	Status DataSyncStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// DataSyncList contains a list of DataSync.
type DataSyncList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DataSync `json:"items"`
}

func init() {
	SchemeBuilder.Register(&DataSync{}, &DataSyncList{})
}
