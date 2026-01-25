package k8s

import (
	"fmt"
	"os"
	"sync"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	kubeClient     kubernetes.Interface
	kubeClientOnce sync.Once
	kubeClientErr  error
)

func GetKubeClient(kubeconfigPath string) (kubernetes.Interface, error) {
	kubeClientOnce.Do(func() {
		var config *rest.Config
		var err error
		if kubeconfigPath != "" {
			config, err = clientcmd.BuildConfigFromFlags("", kubeconfigPath)
		} else {
			config, err = rest.InClusterConfig()
		}
		if err != nil {
			kubeClientErr = fmt.Errorf("failed to build kubernetes config: %w", err)
			return
		}
		kubeClient, err = kubernetes.NewForConfig(config)
		if err != nil {
			kubeClientErr = fmt.Errorf("failed to create kubernetes client: %w", err)
		}
	})
	return kubeClient, kubeClientErr
}

func ResetKubeClient() {
	kubeClient = nil
	kubeClientOnce = sync.Once{}
	kubeClientErr = nil
}

func IsInCluster() bool {
	_, err := rest.InClusterConfig()
	if err != nil {
		return false
	}
	if os.Getenv("KUBERNETES_SERVICE_HOST") == "" {
		return false
	}
	return true
}
