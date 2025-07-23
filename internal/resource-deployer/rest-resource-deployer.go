package resourcedeployer

import (
	"context"

	v1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	"k8s.io/client-go/rest"
	cdiv1beta1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
)

type DataVolumeRestDeployer struct {
	Client *rest.RESTClient
}

func NewDataVolumeRestDeployer(config *rest.Config) (*DataVolumeRestDeployer, error) {
	restClient, err := rest.RESTClientFor(config)

	if err != nil {
		return nil, err
	}

	return &DataVolumeRestDeployer{
		Client: restClient,
	}, nil
}

func (dvrd *DataVolumeRestDeployer) Deploy(
	ctx context.Context,
	dv *cdiv1beta1.DataVolume,
) (*cdiv1beta1.DataVolume, error) {
	createdDV := &cdiv1beta1.DataVolume{}

	err := dvrd.Client.Post().
		Namespace(dv.Namespace).
		Resource("datavolume").
		Body(dv).
		Do(ctx).
		Into(createdDV)

	if err != nil {
		return nil, err
	}

	return createdDV, nil
}

func (dvrd *DataVolumeRestDeployer) Destroy(
	ctx context.Context,
	dv *cdiv1beta1.DataVolume,
) (*cdiv1beta1.DataVolume, error) {
	deletedDv := &cdiv1beta1.DataVolume{}

	err := dvrd.Client.Delete().
		Namespace(dv.Namespace).
		Resource("datavolume").
		Body(dv).
		Do(ctx).
		Into(deletedDv)

	if err != nil {
		return nil, err
	}

	return deletedDv, nil
}

type VolumeSnapshotRestDeployer struct {
	Client *rest.RESTClient
}

func NewVolumeSnapshotRestDeployer(config *rest.Config) (*DataVolumeRestDeployer, error) {
	restClient, err := rest.RESTClientFor(config)

	if err != nil {
		return nil, err
	}

	return &DataVolumeRestDeployer{
		Client: restClient,
	}, nil
}

func (vsrd *VolumeSnapshotRestDeployer) Deploy(
	ctx context.Context,
	vs *v1.VolumeSnapshot,
) (*v1.VolumeSnapshot, error) {
	createdVS := &v1.VolumeSnapshot{}

	err := vsrd.Client.Post().
		Namespace(vs.Namespace).
		Resource("volumesnapshot").
		Body(vs).
		Do(ctx).
		Into(createdVS)

	if err != nil {
		return nil, err
	}

	return createdVS, nil
}

func (vsrd *VolumeSnapshotRestDeployer) Destroy(
	ctx context.Context,
	vs *v1.VolumeSnapshot,
) (*v1.VolumeSnapshot, error) {
	deletedVS := &v1.VolumeSnapshot{}

	err := vsrd.Client.Delete().
		Namespace(vs.Namespace).
		Resource("volumesnapshot").
		Body(vs).
		Do(ctx).
		Into(deletedVS)

	if err != nil {
		return nil, err
	}

	return deletedVS, nil
}
