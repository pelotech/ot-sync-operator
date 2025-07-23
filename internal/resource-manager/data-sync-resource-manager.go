package resourcemanager

import (
	"context"
	crdv1 "pelotech/ot-sync-operator/api/v1"
	resourcedeployer "pelotech/ot-sync-operator/internal/resource-deployer"
	resourcegen "pelotech/ot-sync-operator/internal/resource-generator"

	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	cdiv1beta1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
)


type DataSyncResourceManager struct {
	VolumeSnapshotDeployer resourcedeployer.ResourceDeployer[snapshotv1.VolumeSnapshot]
	DataVolumeDeployer     resourcedeployer.ResourceDeployer[cdiv1beta1.DataVolume]
}

func (dvrm *DataSyncResourceManager) GenerateStorageResources(ctx context.Context, ds *crdv1.DataSync) error {
	for _, resource := range ds.Spec.Resources {
		vs, dv, err := resourcegen.CreateStorageManifestsForDataSyncResource(resource, ds)
		if err != nil {
			return err
		}

		_, err = dvrm.DataVolumeDeployer.Deploy(ctx, dv)

		if err != nil {
			return err
		}

		_, err = dvrm.VolumeSnapshotDeployer.Deploy(ctx, vs)

		if err != nil {
			return err
		}
	}

	return nil
}

func (dvrm *DataSyncResourceManager) TearDownResources(ctx context.Context, ds *crdv1.DataSync) error {
	for _, resource := range ds.Spec.Resources {
		vs, dv, err := resourcegen.CreateStorageManifestsForDataSyncResource(resource, ds)
		if err != nil {
			return err
		}

		_, err = dvrm.DataVolumeDeployer.Destroy(ctx, dv)

		if err != nil {
			return err
		}

		_, err = dvrm.VolumeSnapshotDeployer.Destroy(ctx, vs)

		if err != nil {
			return err
		}
	}

	return nil
}
