package dynamicconfigservice

import (
	"context"
	"fmt"
	crdv1 "pelotech/ot-sync-operator/api/v1"
	generalutils "pelotech/ot-sync-operator/internal/general-utils"
	"strconv"
	"time"

	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"

	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

type OperatorConfig struct {
	Concurrency          int
	RetryLimit           int
	RetryBackoffDuration time.Duration
}

type IDynamicConfigService interface {
	GetOperatorConfig(ctx context.Context, req ctrl.Request) OperatorConfig
}

type DynamicConfigService struct {
	client.Client
	ConfigMapName string
	DefaultConfig OperatorConfig
}

func (dcs *DynamicConfigService) GetOperatorConfig(ctx context.Context, ds crdv1.DataSync) OperatorConfig {
	logger := logf.FromContext(ctx)

	operatorConfigmap, err := generalutils.GetConfigMap(ctx, dcs.Client, dcs.ConfigMapName, ds.Namespace)
	if err != nil {
		logger.Info("No configmap containing operator config found")
		return dcs.DefaultConfig
	}

	config, err := extractOperatorConfig(operatorConfigmap)

	if err != nil {
		logger.Error(err, "Failed to parse dynamic config from the configmap %s", dcs.ConfigMapName)
		return dcs.DefaultConfig
	}

	return *config
}

func extractOperatorConfig(configMap *corev1.ConfigMap) (*OperatorConfig, error) {
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
