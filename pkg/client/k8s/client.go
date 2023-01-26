// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package k8s

import (
	"os"

	"github.com/pkg/errors"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// GetClientSet returns a Kubernetes clientset from the given kubeconfig path.
func GetClientSet(path string) (*kubernetes.Clientset, error) {
	// Load Kubernetes config
	kubeconfigRaw, err := os.ReadFile(path)
	if err != nil {
		return nil, errors.Wrap(err, "could not open kubeconfig file")
	}
	config, err := clientcmd.Load(kubeconfigRaw)
	if err != nil {
		return nil, errors.Wrap(err, "could not load Kubernetes config from kubeconfig file")
	}
	if config == nil {
		return nil, errors.New("Kubernetes config is nil")
	}

	// Create client config
	clientConfig, err := clientcmd.NewDefaultClientConfig(*config, &clientcmd.ConfigOverrides{}).ClientConfig()
	if err != nil {
		return nil, errors.Wrap(err, "could not create client config from Kubernetes config")
	}
	if clientConfig == nil {
		return nil, errors.New("client config is nil")
	}

	// Create clientset
	clientSet, err := kubernetes.NewForConfig(clientConfig)
	if err != nil {
		return nil, errors.Wrap(err, "could not create clientset from client config")
	}
	if clientSet == nil {
		return nil, errors.New("clientset is nil")
	}

	return clientSet, nil
}
