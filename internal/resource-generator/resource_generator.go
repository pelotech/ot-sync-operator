package resourcegenerator

import (
	"errors"

	crdv1 "pelotech/ot-sync-operator/api/v1"

	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	cdiv1beta1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func CreateStorageManifestsForDataSyncResource(
	ds *crdv1.DataSync,
) (*snapshotv1.VolumeSnapshot, *cdiv1beta1.DataVolume, error) {
	volumeSnapshot := createVolumeSnapshot(ds)

	dataVolume, err := createDataVolume(ds)

	if err != nil {
		return nil, nil, err
	}

	return volumeSnapshot, dataVolume, nil
}

func createDataVolume(ds *crdv1.DataSync) (*cdiv1beta1.DataVolume, error) {
	blockOwnerDeletion := true
	ownerReferences := []metav1.OwnerReference{
		{
			APIVersion:         "v1",
			BlockOwnerDeletion: &blockOwnerDeletion,
			Kind:               "DataSync",
			Name:               ds.Name,
			UID:                ds.UID,
		},
	}

	meta := metav1.ObjectMeta{
		Name:            ds.Spec.Name,
		Namespace:       ds.Namespace,
		Labels:          withOperatorLabels(ds.Labels, ds.Name),
		OwnerReferences: ownerReferences,
		Annotations: map[string]string{
			"cdi.kubevirt.io/storage.bind.immediate.requested": "true",
		},
	}

	diskSizeResource, err := resource.ParseQuantity(ds.Spec.DiskSize)

	if err != nil {
		return nil, err
	}

	pvc := &corev1.PersistentVolumeClaimSpec{
		AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
		Resources: corev1.VolumeResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceStorage: diskSizeResource,
			},
		},
	}

	if ds.Spec.StorageClass != nil {
		pvc.StorageClassName = ds.Spec.StorageClass
	}

	var source *cdiv1beta1.DataVolumeSource

	if ds.Spec.SourceType == "s3" {
		source = &cdiv1beta1.DataVolumeSource{
			S3: &cdiv1beta1.DataVolumeSourceS3{
				URL:       ds.Spec.URL,
				SecretRef: ds.Spec.SecretRef,
			},
		}
	} else {
		if ds.Spec.CertConfigMap == nil {
			errMsg := "attempted to create a datavolume without a registry but no certConfigMap was provided"
			return nil, errors.New(errMsg)
		}
		source = &cdiv1beta1.DataVolumeSource{
			Registry: &cdiv1beta1.DataVolumeSourceRegistry{
				URL:           &ds.Spec.URL,
				CertConfigMap: ds.Spec.CertConfigMap,
				SecretRef:     &ds.Spec.SecretRef,
			},
		}
	}

	spec := cdiv1beta1.DataVolumeSpec{
		PVC:    pvc,
		Source: source,
	}

	dv := &cdiv1beta1.DataVolume{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "cdi.kubevirt.io/v1beta1",
			Kind:       "DataVolume",
		},
		ObjectMeta: meta,
		Spec:       spec,
	}

	return dv, nil
}

func createVolumeSnapshot(ds *crdv1.DataSync) *snapshotv1.VolumeSnapshot {
	blockOwnerDeletion := true
	ownerReferences := []metav1.OwnerReference{
		{
			APIVersion:         "v1",
			BlockOwnerDeletion: &blockOwnerDeletion,
			Kind:               "DataSync",
			Name:               ds.Name,
			UID:                ds.UID,
		},
	}

	meta := metav1.ObjectMeta{
		Name:            ds.Spec.Name,
		Namespace:       ds.Namespace,
		Labels:          withOperatorLabels(ds.Labels, ds.Name),
		OwnerReferences: ownerReferences,
	}

	spec := snapshotv1.VolumeSnapshotSpec{
		Source: snapshotv1.VolumeSnapshotSource{
			PersistentVolumeClaimName: &ds.Spec.Name,
		},
	}

	if ds.Spec.SnapshotClass != nil {
		spec.VolumeSnapshotClassName = ds.Spec.SnapshotClass
	}

	return &snapshotv1.VolumeSnapshot{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "snapshot.storage.k8s.io/v1",
			Kind:       "VolumeSnapshot",
		},
		ObjectMeta: meta,
		Spec:       spec,
	}
}

func withOperatorLabels(labels map[string]string, ownerName string) map[string]string {
	labels[crdv1.DataSyncOwnerLabel] = ownerName

	return labels
}
