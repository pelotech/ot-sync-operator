package resourcegenerator

import (
	"errors"
	"fmt"
	"math"
	"regexp"
	"strconv"

	crdv1 "pelotech/ot-sync-operator/api/v1"

	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	cdiv1beta1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func CreateStorageManifestsForDataSyncResource(
	r crdv1.Resource,
	ds *crdv1.DataSync,
) (*snapshotv1.VolumeSnapshot, *cdiv1beta1.DataVolume, error) {
	volumeSnapshot := createVolumeSnapshot(VolumeSnapshotConfig{
		ResourceName: ds.Spec.WorkspaceID,
		Namespace:    ds.Namespace,
		Labels:       ds.Labels,
	})

	dataVolumeConfig := &DataVolumeConfig{
		OwnerUID:     ds.UID,
		OwnerName:    ds.Name,
		ResourceName: ds.Name,
		Namespace:    ds.Namespace,
		Resource:     r,
		Labels:       ds.Labels,
		AddDiskSpace: ds.Spec.AskForDiskSpace,
		SecretRef:    ds.Spec.SecretRef,
	}

	if ds.Spec.CertConfigMap != nil {
		dataVolumeConfig.ConfigMapRef = ds.Spec.CertConfigMap
	}

	if ds.Spec.StorageClass != nil {
		dataVolumeConfig.StorageClass = ds.Spec.StorageClass
	}

	dataVolume, err := createDataVolume(dataVolumeConfig)

	if err != nil {
		return nil, nil, err
	}

	return volumeSnapshot, dataVolume, nil
}

type DataVolumeConfig struct {
	OwnerUID     types.UID
	OwnerName    string
	ResourceName string
	Namespace    string
	Resource     crdv1.Resource
	Labels       map[string]string
	AddDiskSpace bool
	SecretRef    string
	ConfigMapRef *string
	StorageClass *string
}

func createDataVolume(config *DataVolumeConfig) (*cdiv1beta1.DataVolume, error) {
	diskSize, err := calculateDiskSize(config.Resource.DiskSize, config.AddDiskSpace)

	if err != nil {
		return nil, err
	}

	blockOwnerDeletion := true
	ownerReferences := []metav1.OwnerReference{
		{
			APIVersion:         "v1",
			BlockOwnerDeletion: &blockOwnerDeletion,
			Kind:               "DataSync",
			Name:               config.OwnerName,
			UID:                config.OwnerUID,
		},
	}

	meta := metav1.ObjectMeta{
		Name:            config.ResourceName,
		Namespace:       config.Namespace,
		Labels:          config.Labels,
		OwnerReferences: ownerReferences,
		Annotations: map[string]string{
			"cdi.kubevirt.io/storage.bind.immediate.requested": "true",
		},
	}

	pvc := &corev1.PersistentVolumeClaimSpec{
		AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
		Resources: corev1.VolumeResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceStorage: resource.MustParse(diskSize),
			},
		},
	}

	if config.StorageClass != nil {
		pvc.StorageClassName = config.StorageClass
	}

	var source *cdiv1beta1.DataVolumeSource

	if config.Resource.SourceType == "s3" {
		source = &cdiv1beta1.DataVolumeSource{
			S3: &cdiv1beta1.DataVolumeSourceS3{
				URL:       config.Resource.URL,
				SecretRef: config.SecretRef,
			},
		}
	} else {
		if config.ConfigMapRef == nil {
			errMsg := "attempted to create a datavolume without a registry but no certConfigMap was provided"
			return nil, errors.New(errMsg)
		}
		source = &cdiv1beta1.DataVolumeSource{
			Registry: &cdiv1beta1.DataVolumeSourceRegistry{
				URL:           &config.Resource.URL,
				CertConfigMap: config.ConfigMapRef,
				SecretRef:     &config.SecretRef,
			},
		}
	}

	spec := cdiv1beta1.DataVolumeSpec{
		PVC:    pvc,
		Source: source,
	}

	dv := &cdiv1beta1.DataVolume{
		ObjectMeta: meta,
		Spec:       spec,
	}

	return dv, nil
}

type VolumeSnapshotConfig struct {
	ResourceName  string
	Namespace     string
	Labels        map[string]string
	SnapshotClass *string
}

func createVolumeSnapshot(config VolumeSnapshotConfig) *snapshotv1.VolumeSnapshot {
	meta := metav1.ObjectMeta{
		Name:      config.ResourceName,
		Namespace: config.Namespace,
		Labels:    config.Labels,
	}

	spec := snapshotv1.VolumeSnapshotSpec{
		Source: snapshotv1.VolumeSnapshotSource{
			PersistentVolumeClaimName: &config.ResourceName,
		},
	}

	if config.SnapshotClass != nil {
		spec.VolumeSnapshotClassName = config.SnapshotClass
	}

	return &snapshotv1.VolumeSnapshot{
		ObjectMeta: meta,
		Spec:       spec,
	}
}

func calculateDiskSize(diskSize string, addDiskSize bool) (string, error) {
	re := regexp.MustCompile(`([0-9.]+)([a-zA-Z]+)`)

	matches := re.FindStringSubmatch(diskSize)

	if len(matches) != 3 {
		return "", fmt.Errorf("invalid disk size format: %s", diskSize)
	}

	if !addDiskSize {
		return diskSize, nil
	}

	numPart, err := strconv.ParseFloat(matches[1], 64)
	if err != nil {
		return "", err
	}
	unitPart := matches[2]

	augmentedNum := math.Ceil(numPart * 1.33)
	return fmt.Sprintf("%d%s", int(augmentedNum), unitPart), nil
}
