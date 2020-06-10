// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package azure

import (
	"context"
	"strings"
	"time"

	"github.wdf.sap.corp/kubernetes/remedy-controller/pkg/utils/azure"

	aznetwork "github.com/Azure/azure-sdk-for-go/services/network/mgmt/2018-11-01/network"
	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// CleanPublicIps detects and cleans Azure orphan public ips.
func CleanPublicIps(ctx context.Context, k8sClientSet *kubernetes.Clientset, pubipUtils azure.PublicIPAddressUtils, shootName string) {
	var retry int
	for {
		if err := clean(ctx, k8sClientSet, pubipUtils, shootName); err != nil {
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

func clean(ctx context.Context, k8sClientSet *kubernetes.Clientset, pubipUtils azure.PublicIPAddressUtils, shootName string) error {
	// Determine the ips known to Kubernetes
	k8sIps, err := getKnownK8sIps(k8sClientSet)
	if err != nil {
		return err
	}

	// Get all Azure public ips
	azureIps, err := pubipUtils.GetAll(ctx)
	if err != nil {
		return err
	}

	// Separate Azure public ips into orphan and known
	orphanIps, knownIps := separateOrphanAndKnownIps(k8sIps, azureIps, shootName)
	log.Debugf("Count Azure public IPs: %d", len(azureIps))
	log.Infof("Count Kubernetes IPs: %d", len(k8sIps))
	log.Infof("Count orphan public IPs: %d", len(orphanIps))
	log.Infof("Count known public IPs: %d", len(knownIps))

	// Collect the ids of the orphan public ips
	var orphanIpIds []string
	for _, ip := range orphanIps {
		orphanIpIds = append(orphanIpIds, *ip.ID)
	}

	// Remove the orphan public ips from the LoadBalancer
	if err = pubipUtils.RemoveFromLoadBalancer(ctx, orphanIpIds); err != nil {
		return err
	}

	// Remove the orphan public ips
	for _, ip := range orphanIps {
		log.Infof("Removing orphan public IP: %s", *ip.Name)
		if err := pubipUtils.Delete(ctx, *ip.Name); err != nil {
			return err
		}
	}

	log.Info("Removed all orphan public IPs")
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

func separateOrphanAndKnownIps(k8sIps []string, azIps []aznetwork.PublicIPAddress, shootName string) ([]aznetwork.PublicIPAddress, []aznetwork.PublicIPAddress) {
	var orphanIps, knownIps []aznetwork.PublicIPAddress
	for _, azip := range azIps {
		if !strings.Contains(*azip.Name, shootName) {
			continue
		}

		// If an ip resource has no ip assigned then it is invalid and orphan
		if azip.IPAddress == nil {
			log.Debugf("Found orphan public IP: %s", *azip.Name)
			orphanIps = append(orphanIps, azip)
			continue
		}

		// Check if the public ip is also known by Kubernetes
		foundKnownIP := false
		for _, k8sip := range k8sIps {
			if *azip.IPAddress == k8sip {
				log.Debugf("Found known public IP: %s", *azip.Name)
				knownIps = append(knownIps, azip)
				foundKnownIP = true
				break
			}
		}

		// If the public ip is not known by Kubernetes then it is orphan
		if !foundKnownIP {
			log.Debugf("Found orphan public IP: %s", *azip.Name)
			orphanIps = append(orphanIps, azip)
		}
	}
	return orphanIps, knownIps
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
