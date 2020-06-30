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

package service

import (
	"context"
	"time"

	azurev1alpha1 "github.com/gardener/remedy-controller/pkg/apis/azure/v1alpha1"
	"github.com/gardener/remedy-controller/pkg/controller"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	// Label is the label to put on a PublicIPAddress object that identifies its service.
	Label = "azure.remedy.gardener.cloud/service"
)

type actuator struct {
	client    client.Client
	namespace string
	logger    logr.Logger
}

// NewActuator creates a new Actuator.
func NewActuator(client client.Client, namespace string, logger logr.Logger) controller.Actuator {
	logger.Info("Creating actuator", "namespace", namespace)
	return &actuator{
		client:    client,
		namespace: namespace,
		logger:    logger,
	}
}

// CreateOrUpdate reconciles object creation or update.
func (a *actuator) CreateOrUpdate(ctx context.Context, obj runtime.Object) (requeueAfter time.Duration, removeFinalizer bool, err error) {
	// Cast object to Service
	var svc *corev1.Service
	var ok bool
	if svc, ok = obj.(*corev1.Service); !ok {
		return 0, false, errors.New("reconciled object is not a service")
	}

	// Initialize labels
	pubipLabels := map[string]string{
		Label: svc.Namespace + "." + svc.Name,
	}

	// Get LoadBalancer IPs
	ips := getServiceLoadBalancerIPs(svc)

	// Create or update PublicIPAddress objects for existing LoadBalancer IPs
	for ip := range ips {
		pubip := &azurev1alpha1.PublicIPAddress{
			ObjectMeta: metav1.ObjectMeta{
				Name:      generatePublicIPAddressName(svc.Namespace, svc.Name, ip),
				Namespace: a.namespace,
			},
		}
		a.logger.Info("Creating or updating publicipaddress", "name", pubip.Name, "namespace", pubip.Namespace)
		if err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
			_, err := controllerutil.CreateOrUpdate(ctx, a.client, pubip, func() error {
				pubip.Labels = pubipLabels
				pubip.Spec.IPAddress = ip
				return nil
			})
			return err
		}); err != nil {
			return 0, false, errors.Wrap(err, "could not create or update publicipaddress")
		}
	}

	// Delete PublicIPAddress objects for non-existing LoadBalancer IPs
	pubipList := &azurev1alpha1.PublicIPAddressList{}
	if err := a.client.List(ctx, pubipList, client.InNamespace(a.namespace), client.MatchingLabels(pubipLabels)); err != nil {
		return 0, false, errors.Wrap(err, "could not list publicipaddresses")
	}
	for _, pubip := range pubipList.Items {
		if _, ok := ips[pubip.Spec.IPAddress]; !ok {
			a.logger.Info("Deleting publicipaddress", "name", pubip.Name, "namespace", pubip.Namespace)
			if err := client.IgnoreNotFound(a.client.Delete(ctx, &pubip)); err != nil {
				return 0, false, errors.Wrap(err, "could not delete publicipaddress")
			}
		}
	}

	return 0, len(ips) == 0, nil
}

// Delete reconciles object deletion.
func (a *actuator) Delete(ctx context.Context, obj runtime.Object) error {
	// Cast object to Service
	var svc *corev1.Service
	var ok bool
	if svc, ok = obj.(*corev1.Service); !ok {
		return errors.New("reconciled object is not a service")
	}

	// Get LoadBalancer IPs
	ips := getServiceLoadBalancerIPs(svc)

	// Delete PublicIPAddress objects for existing LoadBalancer IPs
	for ip := range ips {
		pubip := &azurev1alpha1.PublicIPAddress{
			ObjectMeta: metav1.ObjectMeta{
				Name:      generatePublicIPAddressName(svc.Namespace, svc.Name, ip),
				Namespace: a.namespace,
			},
		}
		a.logger.Info("Deleting publicipaddress", "name", pubip.Name, "namespace", pubip.Namespace)
		if err := client.IgnoreNotFound(a.client.Delete(ctx, pubip)); err != nil {
			return errors.Wrap(err, "could not delete publicipaddress")
		}
	}

	return nil
}

func getServiceLoadBalancerIPs(svc *corev1.Service) map[string]bool {
	ips := make(map[string]bool)
	if svc.Spec.Type == corev1.ServiceTypeLoadBalancer {
		for _, ingress := range svc.Status.LoadBalancer.Ingress {
			if ingress.IP != "" {
				ips[ingress.IP] = true
			}
		}
	}
	return ips
}

func generatePublicIPAddressName(serviceNamespace, serviceName, ip string) string {
	return serviceNamespace + "-" + serviceName + "-" + ip
}
