package k8s

import (
	"io/ioutil"

	"github.com/pkg/errors"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// GetClientSet returns a Kubernetes clientset from the given kubeconfig path.
func GetClientSet(path string) (*kubernetes.Clientset, error) {
	// Load Kubernetes config
	kubeconfigRaw, err := ioutil.ReadFile(path)
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
