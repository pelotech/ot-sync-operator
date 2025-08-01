package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Condition types and reasons
const (
	DataSyncTypeReady  string = "Ready"
	DataSyncTypeFailed string = "Failed"
)

// DataSync Phases
const (
	DataSyncPhaseQueued    string = "Queued"
	DataSyncPhaseSyncing   string = "Syncing"
	DataSyncPhaseCompleted string = "Completed"
	DataSyncPhaseFailed    string = "Failed"
)

// Datasync Labels
const (
	DataSyncOwnerLabel string = "owner"
)

// Datasync Annotations
const (
	SyncStartTimeAnnotation = "sync-start-time"
)

const DataSyncFinalizer = "pelotech.ot/data-sync-finalizer"

// DataSyncSpec defines the desired state of DataSync.
type DataSyncSpec struct {
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`

	// +kubebuilder:validation:required
	// +kubebuilder:validation:minlength=1
	SecretRef string `json:"secretRef"`

	// +kubebuilder:validation:MinLength=1
	URL string `json:"url"`

	// +kubebuilder:validation:Enum=s3;registry
	SourceType string `json:"sourceType"`

	// DiskSize specifies the size of the disk, e.g., "10Gi", "500Mi".
	// +kubebuilder:validation:Pattern=`^([+-]?[0-9.]+)([eEinumkKMGTP]*[-+]?[0-9]*)$`
	DiskSize string `json:"diskSize"`

	// +kubebuilder:validation:Optional
	StorageClass *string `json:"storageClass,omitempty"`

	// +kubebuilder:validation:Optional
	CertConfigMap *string `json:"certConfigMap,omitempty"`

	// +kubebuilder:validation:Optional
	SnapshotClass *string `json:"snapshotClass,omitempty"`
}

// DataSyncStatus defines the observed state of DataSync.
type DataSyncStatus struct {
	// +kubebuilder:validation:Enum=Queued;Syncing;Completed;Failed
	Phase string `json:"phase"`

	// A human-readable message providing more details about the current phase.
	Message string `json:"message,omitempty"`

	// Conditions of the DataSync resource.
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	FailureCount int `json:"failureCount,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=datasyncs,scope=Namespaced,shortName=ds,singular=datasync
// +kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase",description="The current phase of the DataSync."
// +kubebuilder:printcolumn:name="Resource Name",type="string",JSONPath=".spec.name",description="The name of the resource we are syncing."
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
