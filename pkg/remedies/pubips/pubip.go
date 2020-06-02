package pubips

import (
	"context"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"

	azclient "github.wdf.sap.corp/kubernetes/remedy-controller/pkg/client/azure"

	aznetwork "github.com/Azure/azure-sdk-for-go/services/network/mgmt/2018-11-01/network"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// CleanPubIps TODO
func CleanPubIps(ctx context.Context, k8sClientSet *kubernetes.Clientset, azureClients *azclient.Clients, resourceGroup string) {
	var retry int
	for {
		if err := clean(ctx, k8sClientSet, azureClients, resourceGroup); err != nil {
			log.Error(err.Error())
			time.Sleep(backoff(retry))
			retry++
			continue
		}
		retry = 0

		select {
		case <-time.After(24 * time.Hour):
			continue
		case <-ctx.Done():
			return
		}
	}
}

func clean(ctx context.Context, k8sClientSet *kubernetes.Clientset, azureClients *azclient.Clients, resourceGroup string) error {
	// Determine the Kubernetes known ips.
	k8sIps, err := getKnownK8sIps(k8sClientSet)
	if err != nil {
		return err
	}

	// Determine the public ips on Azure.
	azureIps, err := getIpsFromAzure(ctx, azureClients, resourceGroup)
	if err != nil {
		return err
	}

	var (
		lbName              = resourceGroup
		orphanIps, knownIps = seperatePublicIpsKnownUnknown(k8sIps, azureIps, resourceGroup)
	)
	log.Debugf("Count Azure IPs: %d", len(azureIps))
	log.Infof("Count Kubernetes IPs: %d", len(k8sIps))
	log.Infof("Count orphan IPs: %d", len(orphanIps))
	log.Infof("Count known IPs: %d", len(knownIps))

	// Fetch the LoadBalancer.
	lb, err := azureClients.LoadBalancersClient.Get(ctx, resourceGroup, lbName, "")
	if err != nil {
		return err
	}

	// Determine the valid LoadBalancer FrontendConfigs, LbRules and Health Probes.
	var (
		validFrontendConfigs = determineValidLBFrontendConfigs(lb, knownIps)
		validLBRules         = determineValidLBRules(lb, validFrontendConfigs)
		validLBProbes        = determineValidLBHealthProbes(lb, validLBRules)
	)

	log.Infof("Found %d valid frontend ip configs", len(*validFrontendConfigs))
	log.Infof("Found %d valid lb rules", len(*validLBRules))
	log.Infof("Found %d valid lb health probes", len(*validLBProbes))

	// Update the LoadBalancer only with the valid FrontendConfigs, LbRules and Health Probes.
	lb.FrontendIPConfigurations = validFrontendConfigs
	lb.LoadBalancingRules = validLBRules
	lb.Probes = validLBProbes

	log.Infof("Update LoadBalancer %s", lbName)
	result, err := azureClients.LoadBalancersClient.CreateOrUpdate(ctx, resourceGroup, lbName, lb)
	if err != nil {
		return err
	}
	err = result.WaitForCompletionRef(ctx, azureClients.LoadBalancersClient.Client)
	if err != nil {
		return err
	}

	// Remove the orphan public ips.
	for _, ip := range orphanIps {
		log.Infof("Remove orphan public ip: %s", *ip.Name)
		result, err := azureClients.PublicIPAddressesClient.Delete(ctx, resourceGroup, *ip.Name)
		if err != nil {
			log.Error(err.Error())
			continue
		}
		err = result.WaitForCompletionRef(ctx, azureClients.PublicIPAddressesClient.Client)
		if err != nil {
			return err
		}
	}

	log.Info("Removed all public ips.")
	return nil
}

func getKnownK8sIps(k8sClientSet *kubernetes.Clientset) ([]string, error) {
	services, err := k8sClientSet.CoreV1().Services(metav1.NamespaceAll).List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	var ips []string
	for _, svc := range services.Items {
		if svc.Spec.Type == corev1.ServiceTypeLoadBalancer {
			for _, ing := range svc.Status.LoadBalancer.Ingress {
				ips = append(ips, ing.IP)
			}
		}
	}
	return ips, nil
}

func getIpsFromAzure(ctx context.Context, azureClients *azclient.Clients, resourceGroup string) ([]aznetwork.PublicIPAddress, error) {
	ipList, err := azureClients.PublicIPAddressesClient.List(ctx, resourceGroup)
	if err != nil {
		return nil, err
	}
	var ips []aznetwork.PublicIPAddress
	ips = append(ips, ipList.Values()...)
	return ips, nil
}

func seperatePublicIpsKnownUnknown(k8sIps []string, azIps []aznetwork.PublicIPAddress, shootName string) ([]aznetwork.PublicIPAddress, []aznetwork.PublicIPAddress) {
	var (
		orphanIps    []aznetwork.PublicIPAddress
		knownIps     []aznetwork.PublicIPAddress
		foundKnownIP bool
	)
	for _, azip := range azIps {
		if !strings.Contains(*azip.Name, shootName) {
			continue
		}

		// If an ip resource has no ip assigned then it is invalid and orphan.
		if azip.IPAddress == nil {
			log.Debugf("Found orphan public ip %s", *azip.Name)
			orphanIps = append(orphanIps, azip)
			continue
		}

		// Check if the public ip is also known by Kubernetes.
		for _, k8sip := range k8sIps {
			if *azip.IPAddress == k8sip {
				log.Debugf("Found valid public ip %s", *azip.Name)
				knownIps = append(knownIps, azip)
				foundKnownIP = true
				break
			}
		}

		// Label the public ip as orphan if the ip is not known by Kubernetes.
		if !foundKnownIP {
			log.Debugf("Found orphan public ip %s", *azip.Name)
			orphanIps = append(orphanIps, azip)
		}
		foundKnownIP = false
	}
	return orphanIps, knownIps
}

func determineValidLBFrontendConfigs(lb aznetwork.LoadBalancer, knownIps []aznetwork.PublicIPAddress) *[]aznetwork.FrontendIPConfiguration {
	var frontendConfigList []aznetwork.FrontendIPConfiguration

	for _, fc := range *lb.FrontendIPConfigurations {
		// Remove the frontend ip config if it has no public ip assigned.
		if fc.PublicIPAddress == nil {
			continue
		}

		// Keep only the frontend ip configs which have a known public ip assigned.
		for _, ip := range knownIps {
			if *fc.PublicIPAddress.ID == *ip.ID {
				frontendConfigList = append(frontendConfigList, fc)
			}
		}
	}
	return &frontendConfigList
}

func determineValidLBRules(lb aznetwork.LoadBalancer, frontendConfigs *[]aznetwork.FrontendIPConfiguration) *[]aznetwork.LoadBalancingRule {
	var lbRuleList []aznetwork.LoadBalancingRule

	for _, rule := range *lb.LoadBalancingRules {
		if rule.FrontendIPConfiguration == nil || rule.FrontendIPConfiguration.ID == nil {
			continue
		}

		// Keep only lb rules which valid frontend ip config.
		for _, fc := range *frontendConfigs {
			if *rule.FrontendIPConfiguration.ID == *fc.ID {
				lbRuleList = append(lbRuleList, rule)
			}
		}
	}
	return &lbRuleList
}

func determineValidLBHealthProbes(lb aznetwork.LoadBalancer, rules *[]aznetwork.LoadBalancingRule) *[]aznetwork.Probe {
	var lbHealthProbes []aznetwork.Probe

	for _, probe := range *lb.Probes {
		if probe.LoadBalancingRules == nil || len(*probe.LoadBalancingRules) == 0 {
			continue
		}

		for _, probeRule := range *probe.LoadBalancingRules {
			if probeRule.ID == nil {
				continue
			}

			for _, rule := range *rules {
				if *probeRule.ID == *rule.ID {
					lbHealthProbes = append(lbHealthProbes, probe)
				}
			}
		}
	}
	return &lbHealthProbes
}

func backoff(retry int) time.Duration {
	b := fib(retry)
	if b > 5*time.Minute {
		return 5 * time.Minute
	}
	return b
}

func fib(n int) time.Duration {
	if n <= 1 {
		return time.Second * time.Duration(n)
	}
	return fib(n-1) + fib(n-2)
}
