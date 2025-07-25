package generalutils

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func GetConfigMap(ctx context.Context, c client.Client, configMapName string, namespace string) (*corev1.ConfigMap, error) {
	name := types.NamespacedName{
		Name:      configMapName,
		Namespace: namespace,
	}

	configMap := &corev1.ConfigMap{}
	err := c.Get(ctx, name, configMap)

	if err != nil {
		return nil, fmt.Errorf("failed to get configmap %s: %w", name.Name, err)
	}

	return configMap, nil
}

func GetSecret(ctx context.Context, c client.Client, secretName string, namespace string) (*corev1.Secret, error) {
	name := types.NamespacedName{
		Name:      secretName,
		Namespace: namespace,
	}

	secret := &corev1.Secret{}
	err := c.Get(ctx, name, secret)

	if err != nil {
		return nil, fmt.Errorf("failed to get secret %s: %w", name.Name, err)
	}

	return secret, nil
}
