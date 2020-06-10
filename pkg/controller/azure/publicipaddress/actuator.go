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

package publicipaddress

import (
	"context"
	"time"

	azurev1alpha1 "github.wdf.sap.corp/kubernetes/remedy-controller/pkg/apis/azure/v1alpha1"
	"github.wdf.sap.corp/kubernetes/remedy-controller/pkg/apis/config"
	"github.wdf.sap.corp/kubernetes/remedy-controller/pkg/controller"
	"github.wdf.sap.corp/kubernetes/remedy-controller/pkg/utils/azure"

	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2018-11-01/network"
	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	controllererror "github.com/gardener/gardener/extensions/pkg/controller/error"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type actuator struct {
	client     client.Client
	pubipUtils azure.PublicIPAddressUtils
	config     config.AzurePublicIPRemedyConfiguration
	logger     logr.Logger
}

// NewActuator creates a new Actuator.
func NewActuator(
	pubipUtils azure.PublicIPAddressUtils,
	config config.AzurePublicIPRemedyConfiguration,
	logger logr.Logger,
) controller.Actuator {
	logger.Info("Creating actuator", "config", config)
	return &actuator{
		pubipUtils: pubipUtils,
		config:     config,
		logger:     logger,
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

	// Get the public IP address in Azure
	azurePublicIP, err := a.getAzurePublicIPAddress(ctx, pubip)
	if err != nil {
		return 0, false, err
	}

	// Update resource status
	if err := a.updatePublicIPAddressStatus(ctx, pubip, azurePublicIP); err != nil {
		return 0, false, err
	}

	// Requeue if the public IP address doesn't exist or is in a transient state
	requeueAfter = 0
	if azurePublicIP == nil || (getProvisioningState(azurePublicIP) != network.Succeeded && getProvisioningState(azurePublicIP) != network.Failed) {
		requeueAfter = a.config.RequeueInterval.Duration
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

	// Get the public IP address in Azure
	azurePublicIP, err := a.getAzurePublicIPAddress(ctx, pubip)
	if err != nil {
		return err
	}

	if azurePublicIP != nil {
		// Update resource status
		if err := a.updatePublicIPAddressStatus(ctx, pubip, azurePublicIP); err != nil {
			return err
		}

		// If within a configurable duration after the deletion timestamp, requeue so we could check again
		if pubip.DeletionTimestamp != nil &&
			!time.Now().After(pubip.DeletionTimestamp.Add(a.config.DeletionGracePeriod.Duration)) {
			return &controllererror.RequeueAfterError{
				Cause:        errors.New("public IP address still exists"),
				RequeueAfter: a.config.RequeueInterval.Duration,
			}
		}

		// Clean the public IP address from Azure
		if err := a.cleanAzurePublicIPAddress(ctx, pubip); err != nil {
			return err
		}
	}

	return nil
}

func (a *actuator) getAzurePublicIPAddress(ctx context.Context, pubip *azurev1alpha1.PublicIPAddress) (*network.PublicIPAddress, error) {
	if pubip.Status.Name != nil {
		azurePublicIP, err := a.pubipUtils.GetByName(ctx, *pubip.Status.Name)
		return azurePublicIP, errors.Wrap(err, "could not get Azure public IP address by name")
	}
	azurePublicIP, err := a.pubipUtils.GetByIP(ctx, pubip.Spec.IPAddress)
	return azurePublicIP, errors.Wrap(err, "could not get Azure public IP address by IP")
}

func (a *actuator) cleanAzurePublicIPAddress(ctx context.Context, pubip *azurev1alpha1.PublicIPAddress) error {
	a.logger.Info("Removing Azure public IP address from the load balancer", "id", *pubip.Status.ID)
	if err := a.pubipUtils.RemoveFromLoadBalancer(ctx, []string{*pubip.Status.ID}); err != nil {
		return errors.Wrap(err, "could not remove Azure public IP address from the load balancer")
	}
	a.logger.Info("Deleting Azure public IP address", "name", *pubip.Status.Name)
	return errors.Wrap(a.pubipUtils.Delete(ctx, *pubip.Status.Name), "could not delete Azure public IP address")
}

func (a *actuator) updatePublicIPAddressStatus(ctx context.Context, pubip *azurev1alpha1.PublicIPAddress, azurePublicIP *network.PublicIPAddress) error {
	// Build status
	status := azurev1alpha1.PublicIPAddressStatus{}
	if azurePublicIP != nil {
		status = azurev1alpha1.PublicIPAddressStatus{
			Exists:            true,
			ID:                azurePublicIP.ID,
			Name:              azurePublicIP.Name,
			ProvisioningState: azurePublicIP.ProvisioningState,
		}
	}

	// Update resource status
	a.logger.Info("Updating publicipaddress status", "name", pubip.Name, "namespace", pubip.Namespace, "status", status)
	if err := extensionscontroller.TryUpdateStatus(ctx, retry.DefaultBackoff, a.client, pubip, func() error {
		pubip.Status = status
		return nil
	}); err != nil {
		return errors.Wrap(err, "could not update publicipaddress status")
	}
	return nil
}

func getProvisioningState(azurePublicIP *network.PublicIPAddress) network.ProvisioningState {
	if azurePublicIP.ProvisioningState == nil {
		return ""
	}
	return network.ProvisioningState(*azurePublicIP.ProvisioningState)
}
