package pubips

import (
	"os"

	azclient "github.wdf.sap.corp/kubernetes/azure-remeny-controller/pkg/azure/client"
	"k8s.io/client-go/kubernetes"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CleanPubIps TODO
func CleanPubIps(k8sClientSet *kubernetes.Clientset, azureClients *azclient.AzureDriverClients) {
	svc, err := k8sClientSet.CoreV1().Services(metav1.NamespaceAll).List(metav1.ListOptions{})
	if err != nil {
		os.Exit(1)
	}
	var pubips []string
	for _, s := range svc.Items {
		if s.Spec.Type == "LoadBalancer" {
			for _, ing := range s.Status.LoadBalancer.Ingress {
				pubips = append(pubips, ing.IP)
			}
		}
	}
}
