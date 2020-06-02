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

package service

import (
	"context"
	"net/http"
	"strings"
	"time"

	azurev1alpha1 "github.wdf.sap.corp/kubernetes/remedy-controller/pkg/apis/azure/v1alpha1"
	"github.wdf.sap.corp/kubernetes/remedy-controller/pkg/client/azure"
	"github.wdf.sap.corp/kubernetes/remedy-controller/pkg/controller"

	aznetwork "github.com/Azure/azure-sdk-for-go/services/network/mgmt/2018-11-01/network"
	azautorest "github.com/Azure/go-autorest/autorest"
	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	controllererror "github.com/gardener/gardener/extensions/pkg/controller/error"
	gardenerutils "github.com/gardener/gardener/pkg/utils"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type actuator struct {
	client        client.Client
	azureClients  *azure.Clients
	resourceGroup string
	logger        logr.Logger
}

// NewActuator creates a new Actuator.
func NewActuator(azureClients *azure.Clients, resourceGroup string) controller.Actuator {
	return &actuator{
		azureClients:  azureClients,
		resourceGroup: resourceGroup,
		logger:        log.Log.WithName("azurepublicipaddress-actuator"),
	}
}

func (a *actuator) InjectClient(client client.Client) error {
	a.client = client
	return nil
}

// CreateOrUpdate reconciles object creation or update.
func (a *actuator) CreateOrUpdate(ctx context.Context, obj runtime.Object) (requeueAfter time.Duration, removeFinalizer bool, err error) {
	// Cast object to PublicIPAddress
	var pubip *azurev1alpha1.PublicIPAddress
	var ok bool
	if pubip, ok = obj.(*azurev1alpha1.PublicIPAddress); !ok {
		return 0, false, errors.New("reconciled object is not a publicipaddress")
	}

	// Get IP from name
	ip := getIPFromName(pubip.Name)

	// Get the public IP address in Azure and build resource status
	status, err := a.buildPublicIPAddressStatus(ctx, pubip.Status.Name, ip)
	if err != nil {
		return 0, false, err
	}

	// Update resource status
	if err := a.updatePublicIPAddressStatus(ctx, pubip, status); err != nil {
		return 0, false, err
	}

	requeueAfter = 0
	if !status.Exists || (getProvisioningState(status) != aznetwork.Succeeded && getProvisioningState(status) != aznetwork.Failed) {
		requeueAfter = 30 * time.Second
	}

	return requeueAfter, false, nil
}

// Delete reconciles object deletion.
func (a *actuator) Delete(ctx context.Context, obj runtime.Object) error {
	// Cast object to PublicIPAddress
	var pubip *azurev1alpha1.PublicIPAddress
	var ok bool
	if pubip, ok = obj.(*azurev1alpha1.PublicIPAddress); !ok {
		return errors.New("reconciled object is not a publicipaddress")
	}

	// Get IP from name
	ip := getIPFromName(pubip.Name)

	// Get the public IP address in Azure and build resource status
	status, err := a.buildPublicIPAddressStatus(ctx, pubip.Status.Name, ip)
	if err != nil {
		return err
	}

	if status.Exists {
		// Update resource status
		if err := a.updatePublicIPAddressStatus(ctx, pubip, status); err != nil {
			return err
		}

		// If within a configurable duration after the deletion timestamp, requeue so we could check again
		if pubip.DeletionTimestamp != nil && !time.Now().After(pubip.DeletionTimestamp.Add(5*time.Minute)) {
			return &controllererror.RequeueAfterError{
				Cause:        errors.New("public IP address still exists"),
				RequeueAfter: 30 * time.Second,
			}
		}

		// Clean the public IP address from Azure
		if err := a.cleanAzurePublicIPAddress(ctx, pubip); err != nil {
			return err
		}
	}

	return nil
}

func (a *actuator) buildPublicIPAddressStatus(ctx context.Context, name *string, ip string) (azurev1alpha1.PublicIPAddressStatus, error) {
	// Get the public IP address in Azure and compose resource status
	status := azurev1alpha1.PublicIPAddressStatus{}
	azurePublicIP, err := a.getAzurePublicIPAddress(ctx, name, ip)
	if err != nil {
		return status, err
	}
	if azurePublicIP != nil {
		status = azurev1alpha1.PublicIPAddressStatus{
			Exists:            true,
			ID:                azurePublicIP.ID,
			Name:              azurePublicIP.Name,
			IPAddress:         azurePublicIP.IPAddress,
			ProvisioningState: azurePublicIP.ProvisioningState,
		}
	}
	return status, nil
}

func (a *actuator) getAzurePublicIPAddress(ctx context.Context, name *string, ip string) (*aznetwork.PublicIPAddress, error) {
	if name != nil {
		azurePublicIP, err := a.azureClients.PublicIPAddressesClient.Get(ctx, a.resourceGroup, *name, "")
		if err != nil {
			if isAzureNotFoundError(err) {
				return nil, nil
			}
			return nil, errors.Wrap(err, "could not get Azure public IP address by name")
		}
		return &azurePublicIP, nil
	}

	azurePublicIPList, err := a.azureClients.PublicIPAddressesClient.List(ctx, a.resourceGroup)
	if err != nil {
		return nil, errors.Wrap(err, "could not list Azure public IP addresses")
	}
	for azurePublicIPList.NotDone() {
		for _, azurePublicIP := range azurePublicIPList.Values() {
			if azurePublicIP.IPAddress != nil && *azurePublicIP.IPAddress == ip {
				return &azurePublicIP, nil
			}
		}
		if err := azurePublicIPList.NextWithContext(ctx); err != nil {
			return nil, errors.Wrap(err, "could not advance to the next page of Azure public IP addresses")
		}
	}

	return nil, nil
}

func (a *actuator) updatePublicIPAddressStatus(ctx context.Context, pubip *azurev1alpha1.PublicIPAddress, status azurev1alpha1.PublicIPAddressStatus) error {
	a.logger.Info("Updating publicipaddress status", "name", pubip.Name, "namespace", pubip.Namespace, "status", status)
	if err := extensionscontroller.TryUpdateStatus(ctx, retry.DefaultBackoff, a.client, pubip, func() error {
		pubip.Status = status
		return nil
	}); err != nil {
		return errors.Wrap(err, "could not update publicipaddress status")
	}
	return nil
}

func (a *actuator) cleanAzurePublicIPAddress(ctx context.Context, pubip *azurev1alpha1.PublicIPAddress) error {
	a.logger.Info("Cleaning up Azure public IP address", "name", *pubip.Status.Name)

	// Get the Azure LoadBalancer
	lbName := a.resourceGroup // TODO
	lb, err := a.azureClients.LoadBalancersClient.Get(ctx, a.resourceGroup, lbName, "")
	if err != nil {
		return errors.Wrap(err, "could not get Azure LoadBalancer")
	}

	// Update the FrontendIPConfigurations, LoadBalancerRules, and Probes on the Azure LoadBalancer
	fcIDs := updateFrontendIPConfigurations(lb, pubip.Status.ID)
	ruleIDs := updateLoadBalancingRules(lb, fcIDs)
	updateProbes(lb, ruleIDs)
	a.logger.Info("Updating Azure LoadBalancer", "name", lbName, "lb", lb)
	result, err := a.azureClients.LoadBalancersClient.CreateOrUpdate(ctx, a.resourceGroup, lbName, lb)
	if err != nil {
		return errors.Wrap(err, "could not update Azure LoadBalancer")
	}
	if err := result.WaitForCompletionRef(ctx, a.azureClients.LoadBalancersClient.Client); err != nil {
		return errors.Wrap(err, "could not wait for the Azure LoadBalancer update to complete")
	}

	// Delete the Azure PublicIPAddress
	a.logger.Info("Deleting Azure PublicIPAddress", "name", *pubip.Status.Name)
	deleteResult, err := a.azureClients.PublicIPAddressesClient.Delete(ctx, a.resourceGroup, *pubip.Status.Name)
	if err != nil {
		return errors.Wrap(err, "could not delete Azure PublicIPAddress")
	}
	if err := deleteResult.WaitForCompletionRef(ctx, a.azureClients.PublicIPAddressesClient.Client); err != nil {
		return errors.Wrap(err, "could not wait for the Azure PublicIPAddress deletion to complete")
	}

	return nil
}

func updateFrontendIPConfigurations(lb aznetwork.LoadBalancer, publicIPAddressID *string) []string {
	if publicIPAddressID == nil || lb.FrontendIPConfigurations == nil {
		return nil
	}
	var fcIDs []string
	var updated []aznetwork.FrontendIPConfiguration
	for _, fc := range *lb.FrontendIPConfigurations {
		if fc.ID != nil && fc.PublicIPAddress != nil && fc.PublicIPAddress.ID != nil && *fc.PublicIPAddress.ID == *publicIPAddressID {
			fcIDs = append(fcIDs, *fc.ID)
		} else {
			updated = append(updated, fc)
		}
	}
	*lb.FrontendIPConfigurations = updated
	return fcIDs
}

func updateLoadBalancingRules(lb aznetwork.LoadBalancer, fcIDs []string) []string {
	if lb.LoadBalancingRules == nil {
		return nil
	}
	var ruleIDs []string
	var updated []aznetwork.LoadBalancingRule
	for _, rule := range *lb.LoadBalancingRules {
		if rule.ID != nil && rule.FrontendIPConfiguration != nil && rule.FrontendIPConfiguration.ID != nil && gardenerutils.ValueExists(*rule.FrontendIPConfiguration.ID, fcIDs) {
			ruleIDs = append(ruleIDs, *rule.ID)
		} else {
			updated = append(updated, rule)
		}
	}
	*lb.LoadBalancingRules = updated
	return ruleIDs
}

func updateProbes(lb aznetwork.LoadBalancer, ruleIDs []string) {
	if lb.Probes == nil {
		return
	}
	var updated []aznetwork.Probe
	for _, probe := range *lb.Probes {
		if probe.LoadBalancingRules == nil {
			continue
		}
		for _, probeRule := range *probe.LoadBalancingRules {
			if !(probeRule.ID != nil && gardenerutils.ValueExists(*probeRule.ID, ruleIDs)) {
				updated = append(updated, probe)
			}
		}
	}
	*lb.Probes = updated
}

func getIPFromName(name string) string {
	if index := strings.LastIndex(name, "-"); index >= 0 {
		return name[index+1:]
	}
	return name
}

func getProvisioningState(status azurev1alpha1.PublicIPAddressStatus) aznetwork.ProvisioningState {
	if status.ProvisioningState == nil {
		return ""
	}
	return aznetwork.ProvisioningState(*status.ProvisioningState)
}

func isAzureNotFoundError(err error) bool {
	if e, ok := err.(azautorest.DetailedError); ok {
		return e.StatusCode == http.StatusNotFound
	}
	return false
}
