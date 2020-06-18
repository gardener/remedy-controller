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

	azurev1alpha1 "github.wdf.sap.corp/kubernetes/remedy-controller/pkg/apis/azure/v1alpha1"
	"github.wdf.sap.corp/kubernetes/remedy-controller/pkg/apis/config"
	"github.wdf.sap.corp/kubernetes/remedy-controller/pkg/controller"
	"github.wdf.sap.corp/kubernetes/remedy-controller/pkg/utils/azure"

	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2019-07-01/compute"
	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type actuator struct {
	client              client.Client
	vmUtils             azure.VirtualMachineUtils
	config              config.AzureFailedVMRemedyConfiguration
	logger              logr.Logger
	reappliedVMsCounter prometheus.Counter
}

// NewActuator creates a new Actuator.
func NewActuator(
	vmUtils azure.VirtualMachineUtils,
	config config.AzureFailedVMRemedyConfiguration,
	logger logr.Logger,
	reappliedVMsCounter prometheus.Counter,
) controller.Actuator {
	logger.Info("Creating actuator", "config", config)
	return &actuator{
		vmUtils:             vmUtils,
		config:              config,
		logger:              logger,
		reappliedVMsCounter: reappliedVMsCounter,
	}
}

func (a *actuator) InjectClient(client client.Client) error {
	a.client = client
	return nil
}

// CreateOrUpdate reconciles object creation or update.
func (a *actuator) CreateOrUpdate(ctx context.Context, obj runtime.Object) (requeueAfter time.Duration, removeFinalizer bool, err error) {
	// Cast object to VirtualMachine
	var vm *azurev1alpha1.VirtualMachine
	var ok bool
	if vm, ok = obj.(*azurev1alpha1.VirtualMachine); !ok {
		return 0, false, errors.New("reconciled object is not a virtualmachine")
	}

	// Get the Azure virtual machine
	azureVM, err := a.getAzureVirtualMachine(ctx, vm)
	if err != nil {
		return 0, false, err
	}

	// Update resource status
	if err := a.updateVirtualMachineStatus(ctx, vm, azureVM); err != nil {
		return 0, false, err
	}

	// Requeue if the Azure virtual machine doesn't exist or is in a transient state
	requeueAfter = 0
	if azureVM == nil || (getProvisioningState(azureVM) != compute.ProvisioningStateSucceeded && getProvisioningState(azureVM) != compute.ProvisioningStateFailed) {
		requeueAfter = a.config.RequeueInterval.Duration
	}

	// Reapply the Azure virtual machine if it's in a Failed state
	if azureVM != nil && getProvisioningState(azureVM) == compute.ProvisioningStateFailed {
		if err := a.reapplyAzureVirtualMachine(ctx, vm); err != nil {
			return 0, false, err
		}
	}

	return requeueAfter, false, nil
}

// Delete reconciles object deletion.
func (a *actuator) Delete(ctx context.Context, obj runtime.Object) error {
	// Cast object to VirtualMachine
	var vm *azurev1alpha1.VirtualMachine
	var ok bool
	if vm, ok = obj.(*azurev1alpha1.VirtualMachine); !ok {
		return errors.New("reconciled object is not a virtualmachine")
	}

	// Get the Azure virtual machine
	azureVM, err := a.getAzureVirtualMachine(ctx, vm)
	if err != nil {
		return err
	}

	// Update resource status
	if err := a.updateVirtualMachineStatus(ctx, vm, azureVM); err != nil {
		return err
	}

	return nil
}

func (a *actuator) getAzureVirtualMachine(ctx context.Context, vm *azurev1alpha1.VirtualMachine) (*compute.VirtualMachine, error) {
	azureVM, err := a.vmUtils.Get(ctx, getVirtualMachineName(vm))
	return azureVM, errors.Wrap(err, "could not get Azure virtual machine")
}

func (a *actuator) reapplyAzureVirtualMachine(ctx context.Context, vm *azurev1alpha1.VirtualMachine) error {
	a.logger.Info("Reapplying Azure virtual machine", "name", getVirtualMachineName(vm))
	if err := a.vmUtils.Reapply(ctx, getVirtualMachineName(vm)); err != nil {
		return errors.Wrap(err, "could not reapply Azure virtual machine")
	}
	a.reappliedVMsCounter.Inc()
	return nil
}

func (a *actuator) updateVirtualMachineStatus(ctx context.Context, vm *azurev1alpha1.VirtualMachine, azureVM *compute.VirtualMachine) error {
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

	// Update resource status
	a.logger.Info("Updating virtualmachine status", "name", vm.Name, "status", status)
	if err := extensionscontroller.TryUpdateStatus(ctx, retry.DefaultBackoff, a.client, vm, func() error {
		vm.Status = status
		return nil
	}); err != nil {
		return errors.Wrap(err, "could not update virtualmachine status")
	}
	return nil
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

func getProvisioningState(azureVM *compute.VirtualMachine) compute.ProvisioningState {
	if azureVM.ProvisioningState == nil {
		return ""
	}
	return compute.ProvisioningState(*azureVM.ProvisioningState)
}
