package contollerutils

import (
	"context"
	crdv1 "pelotech/ot-sync-operator/api/v1"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

func ListDataSyncsByPhase(ctx context.Context, c client.Client, phase string) (*crdv1.DataSyncList, error) {
	list := &crdv1.DataSyncList{}

	listOpts := []client.ListOption{
		client.MatchingFields{".status.phase": phase},
	}

	if err := c.List(ctx, list, listOpts...); err != nil {
		return nil, err
	}

	return list, nil
}

func IndexDataSyncByPhase(rawObj client.Object) []string {
	ds, ok := rawObj.(*crdv1.DataSync)

	if !ok {
		return nil
	}

	if ds.Status.Phase == "" {
		return nil
	}

	return []string{ds.Status.Phase}
}
