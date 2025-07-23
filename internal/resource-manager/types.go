package resourcemanager

import "context"

type ResourceManager[T any] interface {
	GenerateResources(ctx context.Context, resource *T) error
	TearDownResources(ctx context.Context, resource *T) error
}

