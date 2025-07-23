package contollerutils

import (
	"context"
	"fmt"
	"math/rand"
	crdv1 "pelotech/ot-sync-operator/api/v1"
	"strconv"
	"time"

	corev1 "k8s.io/api/core/v1"
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

// TODO: Add actual logic to get this done.
func SyncIsComplete(ds *crdv1.DataSync) bool {
	isDone := rand.Float64() > .5

	return isDone
}

// TODO: Add actual logic to get this done.
func SyncErrorOccurred(ds *crdv1.DataSync) error {
	hasError := rand.Float64() > .9

	if !hasError {
		return nil
	}

	return fmt.Errorf("this is a mocked sync error")
}

func ExtractOperatorConfig(configMap *corev1.ConfigMap) (*OperatorConfig, error) {
	var ok bool

	concurrencyStr, ok := configMap.Data["concurrency"]
	if !ok {
		return nil, fmt.Errorf("key 'concurrency' not found in configmap %s", configMap.Name)
	}

	concurrency, err := strconv.Atoi(concurrencyStr)

	if err != nil {
		return nil, fmt.Errorf("failed to parse 'concurrency': %w", err)
	}

	retryLimitStr, ok := configMap.Data["retryLimit"]

	if !ok {
		return nil, fmt.Errorf("key 'retryLimit' not found in configmap %s", configMap.Name)
	}

	retryLimit, err := strconv.Atoi(retryLimitStr)

	if err != nil {
		return nil, fmt.Errorf("failed to parse 'retryLimit': %w", err)
	}

	durationStr, ok := configMap.Data["retryBackoffDuration"]

	if !ok {
		return nil, fmt.Errorf("key 'retryBackoffDuration' not found in configmap %s", configMap.Name)
	}

	duration, err := time.ParseDuration(durationStr)

	if err != nil {
		return nil, fmt.Errorf("failed to parse 'retryBackoffDuration': %w", err)
	}

	return &OperatorConfig{
		RetryBackoffDuration: duration,
		RetryLimit:           retryLimit,
		Concurrency:          concurrency,
	}, nil
}
