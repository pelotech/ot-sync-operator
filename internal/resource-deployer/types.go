package resourcedeployer

import "context"

type ResourceDeployer[T any] interface {
	Deploy(ctx context.Context, item *T) (*T, error)
	Destroy(ctx context.Context, item *T) (*T, error)
}
