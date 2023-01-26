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

package virtualmachine

import (
	"context"
	"strings"
	"time"

	azurev1alpha1 "github.com/gardener/remedy-controller/pkg/apis/azure/v1alpha1"
	"github.com/gardener/remedy-controller/pkg/apis/config"
	"github.com/gardener/remedy-controller/pkg/controller"
	"github.com/gardener/remedy-controller/pkg/utils"
	"github.com/gardener/remedy-controller/pkg/utils/azure"
	utilsprometheus "github.com/gardener/remedy-controller/pkg/utils/prometheus"

	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2019-07-01/compute"
	"github.com/gardener/gardener/pkg/controllerutils"
	controllererror "github.com/gardener/gardener/pkg/controllerutils/reconciler"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// VMStateOK is a constant for an OK state of an Azure virtual machine.
	VMStateOK float64 = 0
	// VMStateFailedWillReapply is a constant for a Failed state of an Azure virtual machine that will be reapplied.
	VMStateFailedWillReapply float64 = 1
	// VMStateFailed is a constant for a Failed state of an Azure virtual machine.
	VMStateFailed float64 = 2
)

type actuator struct {
	client              client.Client
	vmUtils             azure.VirtualMachineUtils
	config              config.AzureFailedVMRemedyConfiguration
	timestamper         utils.Timestamper
	logger              logr.Logger
	reappliedVMsCounter prometheus.Counter
	vmStatesGaugeVec    utilsprometheus.GaugeVec
}

// NewActuator creates a new Actuator.
func NewActuator(
	vmUtils azure.VirtualMachineUtils,
	config config.AzureFailedVMRemedyConfiguration,
	timestamper utils.Timestamper,
	logger logr.Logger,
	reappliedVMsCounter prometheus.Counter,
	vmStatesGaugeVec utilsprometheus.GaugeVec,
) controller.Actuator {
	logger.Info("Creating actuator", "config", config)
	return &actuator{
		vmUtils:             vmUtils,
		config:              config,
		timestamper:         timestamper,
		logger:              logger,
		reappliedVMsCounter: reappliedVMsCounter,
		vmStatesGaugeVec:    vmStatesGaugeVec,
	}
}

func (a *actuator) InjectClient(client client.Client) error {
	a.client = client
	return nil
}

// CreateOrUpdate reconciles object creation or update.
func (a *actuator) CreateOrUpdate(ctx context.Context, obj client.Object) (requeueAfter time.Duration, err error) {
	// Cast object to VirtualMachine
	var vm *azurev1alpha1.VirtualMachine
	var ok bool
	if vm, ok = obj.(*azurev1alpha1.VirtualMachine); !ok {
		return 0, errors.New("reconciled object is not a virtualmachine")
	}

	// Determine VM name
	vmName := getVirtualMachineName(vm)

	// Initialize failed operations from VirtualMachine status
	failedOperations := getFailedOperations(vm)

	// Get the Azure virtual machine
	azureVM, err := a.getAzureVirtualMachine(ctx, vmName)
	if err != nil {
		// Add or update the failed operation
		failedOperation := azurev1alpha1.AddOrUpdateFailedOperation(&failedOperations,
			azurev1alpha1.OperationTypeGetVirtualMachine, err.Error(), a.timestamper.Now())
		a.logger.Error(err, "Getting Azure virtual machine failed", "attempts", failedOperation.Attempts)

		// Update resource status
		if err := a.updateVirtualMachineStatus(ctx, vm, azureVM, failedOperations); err != nil {
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
	azurev1alpha1.DeleteFailedOperation(&failedOperations, azurev1alpha1.OperationTypeGetVirtualMachine)

	// Update resource status
	if err := a.updateVirtualMachineStatus(ctx, vm, azureVM, failedOperations); err != nil {
		return 0, err
	}

	// Reapply the Azure virtual machine if it's in a Failed state
	if azureVM != nil && getProvisioningState(azureVM) == compute.ProvisioningStateFailed {
		// Set VM states gauge to "failed will reapply"
		a.vmStatesGaugeVec.WithLabelValues(vmName).Set(VMStateFailedWillReapply)

		// Reapply the Azure virtual machine
		reappliedAzureVM, err := a.reapplyAzureVirtualMachine(ctx, vmName)
		if err != nil {
			// Add or update the failed operation
			failedOperation := azurev1alpha1.AddOrUpdateFailedOperation(&failedOperations,
				azurev1alpha1.OperationTypeReapplyVirtualMachine, err.Error(), a.timestamper.Now())
			a.logger.Error(err, "Reapplying Azure virtual machine failed", "attempts", failedOperation.Attempts)

			// Update resource status
			if err := a.updateVirtualMachineStatus(ctx, vm, azureVM, failedOperations); err != nil {
				return 0, err
			}

			// If the failed operation has been attempted less than the configured max attempts, requeue with exponential backoff
			if failedOperation.Attempts < a.config.MaxReapplyAttempts {
				return 0, &controllererror.RequeueAfterError{
					Cause:        err,
					RequeueAfter: a.config.RequeueInterval.Duration * (1 << (failedOperation.Attempts - 1)),
				}
			}

			// If the configured max attempts has been reached, set VM states gauge to "failed" and return success
			a.vmStatesGaugeVec.WithLabelValues(vmName).Set(VMStateFailed)
			return a.config.SyncPeriod.Duration, nil
		}
		azurev1alpha1.DeleteFailedOperation(&failedOperations, azurev1alpha1.OperationTypeReapplyVirtualMachine)

		// Increase the reapplied VMs counter
		a.reappliedVMsCounter.Inc()

		// Set VM states gauge to "failed" or "ok" depending on the new Azure virtual machine state
		a.setVMStatesGauge(reappliedAzureVM, vmName)

		// Update resource status
		if err := a.updateVirtualMachineStatus(ctx, vm, reappliedAzureVM, failedOperations); err != nil {
			return 0, err
		}
	} else if azureVM != nil && getProvisioningState(azureVM) != compute.ProvisioningStateFailed {
		// Set VM states gauge to "ok"
		a.vmStatesGaugeVec.WithLabelValues(vmName).Set(VMStateOK)
	} else if azureVM == nil {
		// Delete VM states gauge
		a.vmStatesGaugeVec.DeleteLabelValues(vmName)
	}

	// Requeue if the Azure virtual machine doesn't exist or is in a transient state
	requeueAfter = a.config.SyncPeriod.Duration
	if azureVM == nil || (getProvisioningState(azureVM) != compute.ProvisioningStateSucceeded && getProvisioningState(azureVM) != compute.ProvisioningStateFailed) {
		requeueAfter = a.config.RequeueInterval.Duration
	}

	return requeueAfter, nil
}

// Delete reconciles object deletion.
func (a *actuator) Delete(ctx context.Context, obj client.Object) (requeueAfter time.Duration, err error) {
	// Cast object to VirtualMachine
	var vm *azurev1alpha1.VirtualMachine
	var ok bool
	if vm, ok = obj.(*azurev1alpha1.VirtualMachine); !ok {
		return 0, errors.New("reconciled object is not a virtualmachine")
	}

	// Determine VM name
	vmName := getVirtualMachineName(vm)

	// Initialize failed operations from VirtualMachine status
	failedOperations := getFailedOperations(vm)

	// Get the Azure virtual machine
	azureVM, err := a.getAzureVirtualMachine(ctx, vmName)
	if err != nil {
		// Add or update the failed operation
		failedOperation := azurev1alpha1.AddOrUpdateFailedOperation(&failedOperations,
			azurev1alpha1.OperationTypeGetVirtualMachine, err.Error(), a.timestamper.Now())
		a.logger.Error(err, "Getting Azure virtual machine failed", "attempts", failedOperation.Attempts)

		// Update resource status
		if err := a.updateVirtualMachineStatus(ctx, vm, azureVM, failedOperations); err != nil {
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
	azurev1alpha1.DeleteFailedOperation(&failedOperations, azurev1alpha1.OperationTypeGetVirtualMachine)

	// Set VM states gauge to "failed" or "ok" depending on the Azure virtual machine state
	a.setVMStatesGauge(azureVM, vmName)

	// Update resource status
	return 0, a.updateVirtualMachineStatus(ctx, vm, azureVM, failedOperations)
}

// ShouldFinalize returns true if the object should be finalized.
func (a *actuator) ShouldFinalize(_ context.Context, _ client.Object) (bool, error) {
	return true, nil
}

func (a *actuator) getAzureVirtualMachine(ctx context.Context, name string) (*compute.VirtualMachine, error) {
	azureVM, err := a.vmUtils.Get(ctx, name)
	return azureVM, errors.Wrap(err, "could not get Azure virtual machine")
}

func (a *actuator) reapplyAzureVirtualMachine(ctx context.Context, name string) (*compute.VirtualMachine, error) {
	a.logger.Info("Reapplying Azure virtual machine", "name", name)
	if err := a.vmUtils.Reapply(ctx, name); err != nil {
		return nil, errors.Wrap(err, "could not reapply Azure virtual machine")
	}
	azureVM, err := a.vmUtils.Get(ctx, name)
	if err != nil {
		return nil, errors.Wrap(err, "could not get Azure virtual machine")
	}
	return azureVM, nil
}

func (a *actuator) updateVirtualMachineStatus(
	ctx context.Context,
	vm *azurev1alpha1.VirtualMachine,
	azureVM *compute.VirtualMachine,
	failedOperations []azurev1alpha1.FailedOperation,
) error {
	// Build status
	status := azurev1alpha1.VirtualMachineStatus{}
	if azureVM != nil {
		status = azurev1alpha1.VirtualMachineStatus{
			Exists:            true,
			ID:                azureVM.ID,
			Name:              azureVM.Name,
			ProvisioningState: azureVM.ProvisioningState,
		}
	}
	if len(failedOperations) > 0 {
		status.FailedOperations = make([]azurev1alpha1.FailedOperation, len(failedOperations))
		copy(status.FailedOperations, failedOperations)
	}

	// Update resource status
	a.logger.Info("Updating virtualmachine status", "name", vm.Name, "namespace", vm.Namespace, "status", status)
	controllerutils.GetAndCreateOrMergePatch(ctx, a.client, vm, func() error {
		vm.Status = status
		return nil
	})
	//patch := client.MergeFrom(vm.DeepCopy())
	//vm.Status = status
	//a.client.Status().Patch(ctx, vm, patch)
	return nil
}

func (a *actuator) setVMStatesGauge(azureVM *compute.VirtualMachine, name string) {
	switch {
	case azureVM != nil && getProvisioningState(azureVM) == compute.ProvisioningStateFailed:
		a.vmStatesGaugeVec.WithLabelValues(name).Set(VMStateFailed)
	case azureVM != nil && getProvisioningState(azureVM) != compute.ProvisioningStateFailed:
		a.vmStatesGaugeVec.WithLabelValues(name).Set(VMStateOK)
	case azureVM == nil:
		a.vmStatesGaugeVec.DeleteLabelValues(name)
	}
}

func getVirtualMachineName(vm *azurev1alpha1.VirtualMachine) string {
	if vm.Status.Name != nil {
		return *vm.Status.Name
	}
	lsi := strings.LastIndex(vm.Spec.ProviderID, "/")
	if lsi > 0 {
		return vm.Spec.ProviderID[lsi+1:]
	}
	return vm.Name
}

func getFailedOperations(vm *azurev1alpha1.VirtualMachine) []azurev1alpha1.FailedOperation {
	var failedOperations []azurev1alpha1.FailedOperation
	if len(vm.Status.FailedOperations) > 0 {
		failedOperations = make([]azurev1alpha1.FailedOperation, len(vm.Status.FailedOperations))
		copy(failedOperations, vm.Status.FailedOperations)
	}
	return failedOperations
}

func getProvisioningState(azureVM *compute.VirtualMachine) compute.ProvisioningState {
	if azureVM.ProvisioningState == nil {
		return ""
	}
	return compute.ProvisioningState(*azureVM.ProvisioningState)
}
