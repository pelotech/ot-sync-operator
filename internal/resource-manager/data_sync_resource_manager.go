package resourcemanager

import (
	"context"
	"fmt"
	crdv1 "pelotech/ot-sync-operator/api/v1"
	"time"

	dynamicconfigservice "pelotech/ot-sync-operator/internal/dynamic-config-service"
	resourcegen "pelotech/ot-sync-operator/internal/resource-generator"

	corev1 "k8s.io/api/core/v1"

	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	cdiv1beta1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	crutils "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type DataSyncResourceManager struct {
}

const dataVolumeDonePhase = "Succeeded"

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
	deleteByLabels := getLabelsToMatch(ds)

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

	// If we have a finalizer remove it.
	if crutils.ContainsFinalizer(ds, crdv1.DataSyncFinalizer) {
		crutils.RemoveFinalizer(ds, crdv1.DataSyncFinalizer)
		if err := k8sClient.Update(ctx, ds); err != nil {
			return err
		}
	}

	return nil
}

// This function will check if the datavolumes assoicated with our Datasync
// are ready. Currently we only check if the datavolumes are done syncing in our
// manual process. We do not check if any of the other resources are ready.
func (dsrm *DataSyncResourceManager) ResourcesAreReady(
	ctx context.Context,
	k8sClient client.Client,
	ds *crdv1.DataSync,
) (bool, error) {

	searchLabels := getLabelsToMatch(ds)

	listOps := []client.ListOption{
		searchLabels,
	}

	dataVolumeList := &cdiv1beta1.DataVolumeList{}

	if err := k8sClient.List(ctx, dataVolumeList, listOps...); err != nil {
		return false, fmt.Errorf("failed to list datavolumeswith the datasync %s: %w", ds.Name, err)
	}

	dataVolumesReady := true

	for _, dv := range dataVolumeList.Items {
		if dv.Status.Phase != dataVolumeDonePhase {
			dataVolumesReady = false
			break
		}
	}



	return dataVolumesReady, nil
}

// Check if our resources have errors that would require us to
// scuttle the sync.
func (dsrm *DataSyncResourceManager) ResourcesHaveErrors(
	ctx context.Context,
	k8sClient client.Client,
	config dynamicconfigservice.OperatorConfig,
	ds *crdv1.DataSync,
) error {
	// Check if our datasync has been syncing for too long
	now := time.Now()

	syncStartTimeStr, exists := ds.Annotations[crdv1.SyncStartTimeAnnotation]

	if !exists {
		return fmt.Errorf("the datasync %s does not have a recorded sync start time.", ds.Name)
	}

	syncStartTime, err := time.Parse(time.RFC3339, syncStartTimeStr)

	if err != nil {
		return fmt.Errorf("the datasync %s does not have a parseable sync start time.", ds.Name)
	}

	timeSyncing := now.Sub(syncStartTime)

	if timeSyncing > config.MaxSyncDuration {
		return fmt.Errorf("the datasync %s has been syncing longer than the allowed sync time.", ds.Name)
	}

	searchLabels := getLabelsToMatch(ds)

	listOps := []client.ListOption{
		searchLabels,
	}

	dataVolumeList := &cdiv1beta1.DataVolumeList{}

	if err := k8sClient.List(ctx, dataVolumeList, listOps...); err != nil {
		return fmt.Errorf("failed to list datavolumeswith the datasync %s: %w", ds.Name, err)
	}

	for _, dv := range dataVolumeList.Items {
		if dv.Status.RestartCount >= int32(config.RetryLimit) {
			return fmt.Errorf("a datavolume has restarted more than the max for a sync.")
		}
	}

	return nil
}

func getLabelsToMatch(ds *crdv1.DataSync) client.MatchingLabels {
	labelsToMatch := map[string]string{
		crdv1.DataSyncOwnerLabel:   ds.Name,
		crdv1.DataSyncVersionLabel: ds.Spec.Version,
	}

	return client.MatchingLabels(labelsToMatch)
}
