package contollerutils

import (
	"context"
	"fmt"
	"math/rand"
	crdv1 "pelotech/ot-sync-operator/api/v1"
	"strconv"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

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
		return nil
	}

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

// FetchOperatorConfig retrieves and parses all operator and chart configuration from a ConfigMap used to control operator behavior.
func FetchOperatorConfig(ctx context.Context, c client.Client, configMapName string, namespace string) (*OperatorConfig, error) {
	name := types.NamespacedName{
		Name:      configMapName,
		Namespace: namespace,
	}

	configMap := &corev1.ConfigMap{}
	if err := c.Get(ctx, name, configMap); err != nil {
		return nil, fmt.Errorf("failed to get operator configmap %s: %w", name.Name, err)
	}

	config := &OperatorConfig{}
	var ok bool

	concurrencyStr, ok := configMap.Data["concurrency"]
	if !ok {
		return nil, fmt.Errorf("key 'concurrency' not found in configmap %s", name.Name)
	}

	concurrency, err := strconv.Atoi(concurrencyStr)

	if err != nil {
		return nil, fmt.Errorf("failed to parse 'concurrency': %w", err)
	}

	config.Concurrency = concurrency

	retryLimitStr, ok := configMap.Data["retryLimit"]

	if !ok {
		return nil, fmt.Errorf("key 'retryLimit' not found in configmap %s", name.Name)
	}

	retryLimit, err := strconv.Atoi(retryLimitStr)

	if err != nil {
		return nil, fmt.Errorf("failed to parse 'retryLimit': %w", err)
	}

	config.RetryLimit = retryLimit

	durationStr, ok := configMap.Data["retryBackoffDuration"]

	if !ok {
		return nil, fmt.Errorf("key 'retryBackoffDuration' not found in configmap %s", name.Name)
	}

	duration, err := time.ParseDuration(durationStr)

	if err != nil {
		return nil, fmt.Errorf("failed to parse 'retryBackoffDuration': %w", err)
	}

	config.RetryBackoffDuration = duration

	return config, nil
}
