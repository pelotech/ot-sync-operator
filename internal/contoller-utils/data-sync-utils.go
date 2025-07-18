package contollerutils

import (
	"context"
	"math/rand"

	crdv1 "pelotech/ot-sync-operator/api/v1"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ListDataSyncsByPhase retrieves a list of DataSync resources that are in a specific phase.
// It uses an indexed field for efficient lookups.
func ListDataSyncsByPhase(ctx context.Context, c client.Client, phase string) (*crdv1.DataSyncList, error) {
	// Initialize an empty list to store the results.
	list := &crdv1.DataSyncList{}

	// Set up the list options to filter by the indexed '.status.phase' field.
	listOpts := []client.ListOption{
		client.MatchingFields{".status.phase": phase},
	}

	// Execute the list command.
	if err := c.List(ctx, list, listOpts...); err != nil {
		// If the list fails, return no list and the encountered error.
		return nil, err
	}

	// On success, return the populated list and no error.
	return list, nil
}

// IndexDataSyncByPhase is a helper function for the FieldIndexer.
// It extracts the phase from a DataSync object for indexing.
func IndexDataSyncByPhase(rawObj client.Object) []string {
	// Attempt to cast the object to a DataSync object.
	ds, ok := rawObj.(*crdv1.DataSync)
	if !ok {
		// Not a DataSync object, so we can't index it.
		return nil
	}

	// If the phase is not set, don't add it to the index.
	if ds.Status.Phase == "" {
		return nil
	}

	// Return the phase as a slice of strings for the index.
	return []string{ds.Status.Phase}
}

// TODO: Add actual logic to get this done.
func SyncIsComplete(ds *crdv1.DataSync) bool {
	isDone := rand.Float64() > .5

	return isDone
}
