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

package publicipaddress

import (
	"context"
	"strconv"
	"time"

	azurev1alpha1 "github.com/gardener/remedy-controller/pkg/apis/azure/v1alpha1"
	"github.com/gardener/remedy-controller/pkg/apis/config"
	"github.com/gardener/remedy-controller/pkg/controller"
	controllerazure "github.com/gardener/remedy-controller/pkg/controller/azure"
	"github.com/gardener/remedy-controller/pkg/controller/azure/service"
	"github.com/gardener/remedy-controller/pkg/utils"
	"github.com/gardener/remedy-controller/pkg/utils/azure"

	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2018-11-01/network"
	"github.com/gardener/gardener/pkg/controllerutils"
	controllererror "github.com/gardener/gardener/pkg/controllerutils/reconciler"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// ServiceTag is a tag on an Azure public IP address that identifies the Kubernetes service it belongs to.
	ServiceTag = "service"
)

type actuator struct {
	client            client.Client
	pubipUtils        azure.PublicIPAddressUtils
	config            config.AzureOrphanedPublicIPRemedyConfiguration
	timestamper       utils.Timestamper
	logger            logr.Logger
	cleanedIPsCounter prometheus.Counter
}

// NewActuator creates a new Actuator.
func NewActuator(
	pubipUtils azure.PublicIPAddressUtils,
	config config.AzureOrphanedPublicIPRemedyConfiguration,
	timestamper utils.Timestamper,
	logger logr.Logger,
	cleanedIPsCounter prometheus.Counter,
) controller.Actuator {
	logger.Info("Creating actuator", "config", config)
	return &actuator{
		pubipUtils:        pubipUtils,
		config:            config,
		timestamper:       timestamper,
		logger:            logger,
		cleanedIPsCounter: cleanedIPsCounter,
	}
}

func (a *actuator) InjectClient(client client.Client) error {
	a.client = client
	return nil
}

// CreateOrUpdate reconciles object creation or update.
func (a *actuator) CreateOrUpdate(ctx context.Context, obj client.Object) (requeueAfter time.Duration, err error) {
	// Cast object to PublicIPAddress
	var pubip *azurev1alpha1.PublicIPAddress
	var ok bool
	if pubip, ok = obj.(*azurev1alpha1.PublicIPAddress); !ok {
		return 0, errors.New("reconciled object is not a publicipaddress")
	}

	// Initialize failed operations from PublicIPAddress status
	failedOperations := getFailedOperations(pubip)

	// Get the Azure public IP address
	azurePublicIP, err := a.getAzurePublicIPAddress(ctx, pubip)
	if err != nil {
		// Add or update the failed operation
		failedOperation := azurev1alpha1.AddOrUpdateFailedOperation(&failedOperations,
			azurev1alpha1.OperationTypeGetPublicIPAddress, err.Error(), a.timestamper.Now())
		a.logger.Error(err, "Getting Azure public IP address failed", "attempts", failedOperation.Attempts)

		// Update resource status
		if err := a.updatePublicIPAddressStatus(ctx, pubip, azurePublicIP, failedOperations); err != nil {
			return 0, err
		}

		// If the failed operation has been attempted less than the configured max attempts, requeue with exponential backoff
		if failedOperation.Attempts < a.config.MaxGetAttempts {
			return 0, &controllererror.RequeueAfterError{
				Cause:        err,
				RequeueAfter: a.config.RequeueInterval.Duration * (1 << (failedOperation.Attempts - 1)),
			}
		}
		return a.config.SyncPeriod.Duration, nil
	}
	azurev1alpha1.DeleteFailedOperation(&failedOperations, azurev1alpha1.OperationTypeGetPublicIPAddress)

	// Update resource status
	if err := a.updatePublicIPAddressStatus(ctx, pubip, azurePublicIP, failedOperations); err != nil {
		return 0, err
	}

	// Requeue if the Azure public IP address doesn't exist or is in a transient state
	requeueAfter = a.config.SyncPeriod.Duration
	if azurePublicIP == nil || (getProvisioningState(azurePublicIP) != network.Succeeded && getProvisioningState(azurePublicIP) != network.Failed) {
		requeueAfter = a.config.RequeueInterval.Duration
	}

	return requeueAfter, nil
}

// Delete reconciles object deletion.
func (a *actuator) Delete(ctx context.Context, obj client.Object) (requeueAfter time.Duration, err error) {
	// Cast object to PublicIPAddress
	var pubip *azurev1alpha1.PublicIPAddress
	var ok bool
	if pubip, ok = obj.(*azurev1alpha1.PublicIPAddress); !ok {
		return 0, errors.New("reconciled object is not a publicipaddress")
	}

	// Initialize failed operations from PublicIPAddress status
	failedOperations := getFailedOperations(pubip)

	// Get the Azure public IP address
	azurePublicIP, err := a.getAzurePublicIPAddress(ctx, pubip)
	if err != nil {
		// Add or update the failed operation
		failedOperation := azurev1alpha1.AddOrUpdateFailedOperation(&failedOperations,
			azurev1alpha1.OperationTypeGetPublicIPAddress, err.Error(), a.timestamper.Now())
		a.logger.Error(err, "Getting Azure public IP address failed", "attempts", failedOperation.Attempts)

		// Update resource status
		if err := a.updatePublicIPAddressStatus(ctx, pubip, azurePublicIP, failedOperations); err != nil {
			return 0, err
		}

		// If the failed operation has been attempted less than the configured max attempts, requeue with exponential backoff
		if failedOperation.Attempts < a.config.MaxGetAttempts {
			return 0, &controllererror.RequeueAfterError{
				Cause:        err,
				RequeueAfter: a.config.RequeueInterval.Duration * (1 << (failedOperation.Attempts - 1)),
			}
		}
		return a.config.SyncPeriod.Duration, nil
	}
	azurev1alpha1.DeleteFailedOperation(&failedOperations, azurev1alpha1.OperationTypeGetPublicIPAddress)

	// Update resource status
	if err := a.updatePublicIPAddressStatus(ctx, pubip, azurePublicIP, failedOperations); err != nil {
		return 0, err
	}

	// Clean the Azure public IP address if it still exists and the deletion grace period has elapsed
	if azurePublicIP != nil && !shouldNotClean(pubip) {
		// If within the deletion grace period, requeue so we could check again
		if pubip.DeletionTimestamp != nil &&
			!a.timestamper.Now().After(pubip.DeletionTimestamp.Add(a.config.DeletionGracePeriod.Duration)) {
			return 0, &controllererror.RequeueAfterError{
				Cause:        errors.New("public IP address still exists"),
				RequeueAfter: a.config.RequeueInterval.Duration,
			}
		}

		// Clean the Azure public IP address
		if err := a.cleanAzurePublicIPAddress(ctx, pubip); err != nil {
			// Add or update the failed operation
			failedOperation := azurev1alpha1.AddOrUpdateFailedOperation(&failedOperations,
				azurev1alpha1.OperationTypeCleanPublicIPAddress, err.Error(), a.timestamper.Now())
			a.logger.Error(err, "Cleaning Azure public IP address failed", "attempts", failedOperation.Attempts)

			// Update resource status
			if err := a.updatePublicIPAddressStatus(ctx, pubip, azurePublicIP, failedOperations); err != nil {
				return 0, err
			}

			// If the failed operation has been attempted less than the configured max attempts, requeue with exponential backoff
			if failedOperation.Attempts < a.config.MaxCleanAttempts {
				return 0, &controllererror.RequeueAfterError{
					Cause:        err,
					RequeueAfter: a.config.RequeueInterval.Duration * (1 << (failedOperation.Attempts - 1)),
				}
			}
			return a.config.SyncPeriod.Duration, nil
		}
		azurev1alpha1.DeleteFailedOperation(&failedOperations, azurev1alpha1.OperationTypeCleanPublicIPAddress)

		// Increase the cleaned IPs counter
		a.cleanedIPsCounter.Inc()

		// Update resource status
		if err := a.updatePublicIPAddressStatus(ctx, pubip, nil, failedOperations); err != nil {
			return 0, err
		}
	}

	return 0, nil
}

// ShouldFinalize returns true if the object should be finalized.
func (a *actuator) ShouldFinalize(_ context.Context, _ client.Object) (bool, error) {
	return true, nil
}

func (a *actuator) getAzurePublicIPAddress(ctx context.Context, pubip *azurev1alpha1.PublicIPAddress) (*network.PublicIPAddress, error) {
	// If status.name is initialized, search by name
	if pubip.Status.Name != nil {
		azurePublicIP, err := a.pubipUtils.GetByName(ctx, *pubip.Status.Name)
		if err != nil {
			return nil, errors.Wrap(err, "could not get Azure public IP address by name")
		}

		// If an Azure public IP address is found, compare its IP to the PublicIPAddress IP and return it only if there is a match
		if azurePublicIP != nil && *azurePublicIP.IPAddress == pubip.Spec.IPAddress {
			return azurePublicIP, nil
		}
	}

	// Search by IP
	azurePublicIP, err := a.pubipUtils.GetByIP(ctx, pubip.Spec.IPAddress)
	if err != nil {
		return nil, errors.Wrap(err, "could not get Azure public IP address by IP")
	}

	// If an Azure public IP address is found, compare its service tag to the PublicIPAddress service name and return it only if there is a match
	serviceName := service.ObjectLabeler.GetNamespacedName(pubip.Labels[controllerazure.ServiceLabel]).String()
	if azurePublicIP != nil && (serviceName == "/" || azurePublicIP.Tags[ServiceTag] != nil && *azurePublicIP.Tags[ServiceTag] == serviceName) {
		return azurePublicIP, nil
	}

	return nil, nil
}

func (a *actuator) cleanAzurePublicIPAddress(ctx context.Context, pubip *azurev1alpha1.PublicIPAddress) error {
	a.logger.Info("Removing Azure public IP address from the load balancer", "id", *pubip.Status.ID)
	if err := a.pubipUtils.RemoveFromLoadBalancer(ctx, []string{*pubip.Status.ID}); err != nil {
		return errors.Wrap(err, "could not remove Azure public IP address from the load balancer")
	}
	a.logger.Info("Deleting Azure public IP address", "name", *pubip.Status.Name)
	if err := a.pubipUtils.Delete(ctx, *pubip.Status.Name); err != nil {
		return errors.Wrap(err, "could not delete Azure public IP address")
	}
	return nil
}

func (a *actuator) updatePublicIPAddressStatus(
	ctx context.Context,
	pubip *azurev1alpha1.PublicIPAddress,
	azurePublicIP *network.PublicIPAddress,
	failedOperations []azurev1alpha1.FailedOperation,
) error {
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
	if len(failedOperations) > 0 {
		status.FailedOperations = make([]azurev1alpha1.FailedOperation, len(failedOperations))
		copy(status.FailedOperations, failedOperations)
	}

	// Update resource status
	a.logger.Info("Updating publicipaddress status", "name", pubip.Name, "namespace", pubip.Namespace, "status", status)

	if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, a.client, pubip, func() error {
		pubip.Status = status
		return nil
	}); client.IgnoreNotFound(err) != nil {
		return errors.Wrap(err, "could not update publicipaddress status")
	}
	return nil
}

func getFailedOperations(pubip *azurev1alpha1.PublicIPAddress) []azurev1alpha1.FailedOperation {
	var failedOperations []azurev1alpha1.FailedOperation
	if len(pubip.Status.FailedOperations) > 0 {
		failedOperations = make([]azurev1alpha1.FailedOperation, len(pubip.Status.FailedOperations))
		copy(failedOperations, pubip.Status.FailedOperations)
	}
	return failedOperations
}

func shouldNotClean(pubip *azurev1alpha1.PublicIPAddress) bool {
	return pubip.Annotations[controllerazure.DoNotCleanAnnotation] == strconv.FormatBool(true)
}

func getProvisioningState(azurePublicIP *network.PublicIPAddress) network.ProvisioningState {
	if azurePublicIP.ProvisioningState == nil {
		return ""
	}
	return network.ProvisioningState(*azurePublicIP.ProvisioningState)
}
