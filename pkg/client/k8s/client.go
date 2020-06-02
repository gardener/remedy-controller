package k8s

import (
	"fmt"
	"io/ioutil"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// GetClientSet TODO
func GetClientSet(path string) (*kubernetes.Clientset, error) {
	kubeconfigRaw, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	configObj, err := clientcmd.Load(kubeconfigRaw)
	if err != nil {
		return nil, err
	} else if configObj == nil {
		return nil, fmt.Errorf("config object is nil")
	}

	config, err := clientcmd.NewDefaultClientConfig(*configObj, &clientcmd.ConfigOverrides{}).ClientConfig()
	if err != nil {
		return nil, err
	} else if config == nil {
		return nil, fmt.Errorf("client config is nil")
	}

	clientSet, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	} else if clientSet == nil {
		return nil, fmt.Errorf("clientset is nil")
	}

	return clientSet, nil
}
