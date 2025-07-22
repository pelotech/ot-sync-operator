package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// VMS represents a single virtual machine to be synced.
type VM struct {
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`

	// +kubebuilder:validation:MinLength=1
	URL string `json:"url"`

	// +kubebuilder:validation:Enum=s3;registry
	SourceType string `json:"sourceType"`

	// DiskSize specifies the size of the disk, e.g., "10Gi", "500Mi".
	// +kubebuilder:validation:Pattern=`^[0-9.]+[A-Za-z]+$`
	DiskSize string `json:"diskSize"`
}

type CredentialsSecret struct {
	Create      bool   `json:"create"`
	Name        string `json:"name"`
	AccessKeyId string `json:"accessKeyId"`
	SecretKey   string `json:"secretKey"`
}

type CertConfigMap struct {
	Create bool   `json:"create"`
	Name   string `json:"name"`
	Value  string `json:"value"`
}

// TODO: It looks like this is kind of a one off thing that we do in the beginning
//
//	including these on every datasync will be a pain. This need to go somewhere
//	hit up Sean about what we want to do with this stuff
type Auth struct {
	CredentialsSecret `json:"credentialsSecret"`
	CertConfigMap     `json:"certConfigMap"`
}

// DataSyncSpec defines the desired state of DataSync.
type DataSyncSpec struct {
	// The unique identifier for the workspace to be synced.
	// +kubebuilder:validation:MinLength=1
	WorkspaceID string `json:"workspaceId"`

	// +kubebuilder:validation:minlength=1
	Version string `json:"version"`

	AskForDiskSpace bool `json:"askForDiskSpace"`

	// +kubebuilder:validation:minlength=1
	SecretRef string `json:"secretRef"`

	// VMS is a list of virtual machines to be synced.
	// Each VM is identified by a name, URL, and sourceType.
	// +kubebuilder:validation:MinItems=0
	Vms []VM `json:"vms"`

	StorageClass  *string `json:"storageClass,omitempty"`
	CertConfigMap *string `json:"certConfigMap,omitempty"`
}

// DataSyncStatus defines the observed state of DataSync.
type DataSyncStatus struct {
	// +kubebuilder:validation:Enum=Queued;Syncing;Completed;Failed
	Phase string `json:"phase"`

	// A human-readable message providing more details about the current phase.
	Message string `json:"message,omitempty"`

	// Conditions of the DataSync resource.
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// Condition types and reasons
const (
	DataSyncTypeReady string = "Ready"
)

// DataSync Phases
const (
	DataSyncPhaseQueued    string = "Queued"
	DataSyncPhaseSyncing   string = "Syncing"
	DataSyncPhaseCompleted string = "Completed"
	DataSyncPhaseFailed    string = "Failed"
)

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
