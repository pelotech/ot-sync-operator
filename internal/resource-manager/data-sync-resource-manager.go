package resourcemanager

import (
	"context"
	crdv1 "pelotech/ot-sync-operator/api/v1"

	corev1 "k8s.io/api/core/v1"

	resourcegen "pelotech/ot-sync-operator/internal/resource-generator"

	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	cdiv1beta1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type DataSyncResourceManager struct {
}

// Create resources for a given datasync. Stops creating them if
// a single resource fails to create. Does not cleanup after itself
func (dvrm *DataSyncResourceManager) CreateResources(
	ctx context.Context,
	k8sClient client.Client,
	ds *crdv1.DataSync,
) error {
	for _, resource := range ds.Spec.Resources {
		vs, dv, err := resourcegen.CreateStorageManifestsForDataSyncResource(resource, ds)
		if err != nil {
			return err
		}

		err = k8sClient.Patch(ctx, dv, client.Apply, client.FieldOwner("data-sync-operator"))

		if err != nil {
			return err
		}

		err = k8sClient.Patch(ctx, vs, client.Apply, client.FieldOwner("data-sync-operator"))

		if err != nil {
			return err
		}
	}

	return nil
}

// Tear down the resources associated with a given DataSync.
func (dvrm *DataSyncResourceManager) TearDownAllResources(
	ctx context.Context,
	k8sClient client.Client,
	ds *crdv1.DataSync,
) error {
	labelsToMatch := map[string]string{
		crdv1.DataSyncOwnerLabel:   ds.Name,
		crdv1.DataSyncVersionLabel: ds.Spec.Version,
	}

	deleteByLabels := client.MatchingLabels(labelsToMatch)

	// First we tear down the PVCs that back the data volumes
	err := k8sClient.DeleteAllOf(
		ctx,
		&corev1.PersistentVolumeClaim{},
		client.InNamespace(ds.Namespace),
		deleteByLabels,
	)

	if err != nil {
		return err
	}

	// Next the Datavolumes
	err = k8sClient.DeleteAllOf(
		ctx,
		&cdiv1beta1.DataVolume{},
		client.InNamespace(ds.Namespace),
		deleteByLabels,
	)

	if err != nil {
		return err
	}

	// Finally the volumesnapshots
	err = k8sClient.DeleteAllOf(
		ctx,
		&snapshotv1.VolumeSnapshot{},
		client.InNamespace(ds.Namespace),
		deleteByLabels,
	)

	if err != nil {
		return err
	}

	return nil
}
