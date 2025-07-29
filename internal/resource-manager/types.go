package resourcemanager

import (
	"context"

	dynamicconfigservice "pelotech/ot-sync-operator/internal/dynamic-config-service"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ResourceManager[T any] interface {
	CreateResources(ctx context.Context, k8sClient client.Client, resource *T) error
	TearDownAllResources(ctx context.Context, k8sClient client.Client, resource *T) error
	ResourcesAreReady(ctx context.Context, k8sClient client.Client, resource *T) (bool, error)
	ResourcesHaveErrors(
		ctx context.Context,
		k8sClient client.Client,
		config dynamicconfigservice.OperatorConfig,
		resource *T,
	) error
}
