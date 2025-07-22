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
	hasError := rand.Float64() > .1

	if !hasError {
		return nil
	}

	return fmt.Errorf("this is a mocked sync error")
}

func GetConfigMap(ctx context.Context, c client.Client, configMapName string, namespace string) (*corev1.ConfigMap, error) {
	name := types.NamespacedName{
		Name:      configMapName,
		Namespace: namespace,
	}

	configMap := &corev1.ConfigMap{}
	err := c.Get(ctx, name, configMap)

	if err != nil {
		return nil, fmt.Errorf("failed to get operator configmap %s: %w", name.Name, err)
	}

	return configMap, nil
}

func GetSecret(ctx context.Context, c client.Client, configMapName string, namespace string) (*corev1.Secret, error) {
	name := types.NamespacedName{
		Name:      configMapName,
		Namespace: namespace,
	}

	secret := &corev1.Secret{}
	err := c.Get(ctx, name, secret)

	if err != nil {
		return nil, fmt.Errorf("failed to get operator configmap %s: %w", name.Name, err)
	}

	return secret, nil
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
