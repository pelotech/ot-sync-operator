package kubectlclient

import (
	"fmt"
	"os"
	"path/filepath"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

// A function which will create wrapper around Kubectl
func LoadKubectlConfig(inCluster bool) (*rest.Config, error) {

	if inCluster {
		return rest.InClusterConfig()
	}

	homePath := homedir.HomeDir()

	if homePath == "" {
		return nil, fmt.Errorf("failed to resolve home directory when constructing KubectlClient")
	}

	kubeconfigPath := filepath.Join(homePath, ".kube", "config")

	_, err := os.Stat(kubeconfigPath)

	if err != nil {
		return nil, fmt.Errorf("failed to resolve Kubconfig files at %s", kubeconfigPath)
	}

	return clientcmd.BuildConfigFromFlags("", kubeconfigPath)
}
