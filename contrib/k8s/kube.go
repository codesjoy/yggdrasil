// Copyright 2022 The codesjoy Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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

// GetKubeClient returns a kubernetes client.
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

// ResetKubeClient resets the kubernetes client.
func ResetKubeClient() {
	kubeClient = nil
	kubeClientOnce = sync.Once{}
	kubeClientErr = nil
}

// IsInCluster returns true if the program is running in a Kubernetes cluster.
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
