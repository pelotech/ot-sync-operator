package resourcemanager

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ResourceManager[T any] interface {
	CreateResources(ctx context.Context, k8sClient client.Client, resource *T) error
	TearDownAllResources(ctx context.Context, k8sClient client.Client, resource *T) error
}
