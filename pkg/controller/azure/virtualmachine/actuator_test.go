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

package virtualmachine_test

import (
	"context"
	"time"

	azurev1alpha1 "github.com/gardener/remedy-controller/pkg/apis/azure/v1alpha1"
	"github.com/gardener/remedy-controller/pkg/apis/config"
	"github.com/gardener/remedy-controller/pkg/controller"
	"github.com/gardener/remedy-controller/pkg/controller/azure/virtualmachine"
	mockclient "github.com/gardener/remedy-controller/pkg/mock/controller-runtime/client"
	mockprometheus "github.com/gardener/remedy-controller/pkg/mock/prometheus"
	mockutilsazure "github.com/gardener/remedy-controller/pkg/mock/remedy-controller/utils/azure"
	mockutilsprometheus "github.com/gardener/remedy-controller/pkg/mock/remedy-controller/utils/prometheus"
	"github.com/gardener/remedy-controller/pkg/utils"

	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2019-07-01/compute"
	controllererror "github.com/gardener/gardener/pkg/controllerutils/reconciler"
	"github.com/go-logr/logr"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/runtime/inject"
)

type PrometheusMockClient struct {
	*mockutilsprometheus.MockGaugeVec
	gauge                   *mockprometheus.MockGauge
	azureVirtualMachineName string
}

func (c *PrometheusMockClient) ExpectSetGaugeOK() {
	c.ExpectSetGauge(virtualmachine.VMStateOK)
}

func (c *PrometheusMockClient) ExpectSetGauge(state float64) {
	c.EXPECT().WithLabelValues(c.azureVirtualMachineName).Return(c.gauge)
	c.gauge.EXPECT().Set(state)
}

func (c *PrometheusMockClient) ExpectDeleteGauge() {
	c.EXPECT().DeleteLabelValues(c.azureVirtualMachineName)
}

type AzureMockClient struct {
	*mockutilsazure.MockVirtualMachineUtils
	azureVirtualMachineID   string
	azureVirtualMachineName string
}

func NewAzureMockClient(mock *mockutilsazure.MockVirtualMachineUtils, azureVirtualMachineName, azureVirtualMachineID string) *AzureMockClient {
	return &AzureMockClient{
		MockVirtualMachineUtils: mock,
		azureVirtualMachineName: azureVirtualMachineName,
		azureVirtualMachineID:   azureVirtualMachineID,
	}
}

func (c *AzureMockClient) ExpectGetObj(returnProvisioningState compute.ProvisioningState) {
	azureVirtualMachine := &compute.VirtualMachine{
		ID:   pointer.StringPtr(c.azureVirtualMachineID),
		Name: pointer.StringPtr(c.azureVirtualMachineName),
		VirtualMachineProperties: &compute.VirtualMachineProperties{
			ProvisioningState: pointer.StringPtr(string(returnProvisioningState)),
		},
	}
	c.EXPECT().Get(gomock.Any(), c.azureVirtualMachineName).Return(azureVirtualMachine, nil)
}

var _ = Describe("Actuator", func() {
	const (
		nodeName                = "shoot--dev--test-vm1"
		namespace               = "test"
		hostname                = "shoot--dev--test-vm1"
		providerID              = "azure:///subscriptions/xxx/resourceGroups/shoot--dev--test/providers/Microsoft.Compute/virtualMachines/shoot--dev--test-vm1"
		azureVirtualMachineID   = "/subscriptions/xxx/resourceGroups/shoot--dev--test/providers/Microsoft.Compute/virtualMachines/shoot--dev--test-vm1"
		azureVirtualMachineName = "shoot--dev--test-vm1"

		requeueInterval = 1 * time.Second
		syncPeriod      = 1 * time.Minute
	)

	var (
		ctrl      *gomock.Controller
		ctx       context.Context
		azureMock *AzureMockClient
		promMock  *PrometheusMockClient
		c         *mockclient.MockClient

		vmUtils             *mockutilsazure.MockVirtualMachineUtils
		reappliedVMsCounter *mockprometheus.MockCounter

		cfg         config.AzureFailedVMRemedyConfiguration
		now         metav1.Time
		timestamper utils.Timestamper
		logger      logr.Logger
		actuator    controller.Actuator

		newVmFromState               func(state compute.ProvisioningState) *azurev1alpha1.VirtualMachine
		newVM                        func(bool, bool, compute.ProvisioningState, []azurev1alpha1.FailedOperation) *azurev1alpha1.VirtualMachine
		expectPatchStatus            func(vm, vmUpdated *azurev1alpha1.VirtualMachine) *gomock.Call
		expectPatchProvisioningState func(currentState, patchedState compute.ProvisioningState) *gomock.Call
		expectAzStateAndPromUpdate   func(azState compute.ProvisioningState)
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		ctx = context.TODO()

		c = mockclient.NewMockClient(ctrl)
		sw := mockclient.NewMockStatusWriter(ctrl)
		c.EXPECT().Status().Return(sw).AnyTimes()
		vmUtils = mockutilsazure.NewMockVirtualMachineUtils(ctrl)
		azureMock = NewAzureMockClient(vmUtils, azureVirtualMachineName, azureVirtualMachineID)

		reappliedVMsCounter = mockprometheus.NewMockCounter(ctrl)
		vmStatesGaugeVec := mockutilsprometheus.NewMockGaugeVec(ctrl)
		vmStatesGauge := mockprometheus.NewMockGauge(ctrl)
		promMock = &PrometheusMockClient{
			MockGaugeVec:            vmStatesGaugeVec,
			gauge:                   vmStatesGauge,
			azureVirtualMachineName: azureVirtualMachineName,
		}

		cfg = config.AzureFailedVMRemedyConfiguration{
			RequeueInterval:    metav1.Duration{Duration: requeueInterval},
			SyncPeriod:         metav1.Duration{Duration: syncPeriod},
			MaxGetAttempts:     2,
			MaxReapplyAttempts: 2,
		}
		now = metav1.Now()
		timestamper = utils.TimestamperFunc(func() metav1.Time { return now })
		logger = log.Log.WithName("test")
		actuator = virtualmachine.NewActuator(vmUtils, cfg, timestamper, logger, reappliedVMsCounter, vmStatesGaugeVec)
		Expect(actuator.(inject.Client).InjectClient(c)).To(Succeed())

		newVM = func(notReadyOrUnreachable, withStatus bool, provisioningState compute.ProvisioningState, failedOperations []azurev1alpha1.FailedOperation) *azurev1alpha1.VirtualMachine {
			var status azurev1alpha1.VirtualMachineStatus
			if withStatus {
				status = azurev1alpha1.VirtualMachineStatus{
					Exists:            true,
					ID:                pointer.StringPtr(azureVirtualMachineID),
					Name:              pointer.StringPtr(azureVirtualMachineName),
					ProvisioningState: pointer.StringPtr(string(provisioningState)),
				}
			}
			status.FailedOperations = failedOperations
			return &azurev1alpha1.VirtualMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      nodeName,
					Namespace: namespace,
				},
				Spec: azurev1alpha1.VirtualMachineSpec{
					Hostname:              hostname,
					ProviderID:            providerID,
					NotReadyOrUnreachable: notReadyOrUnreachable,
				},
				Status: status,
			}
		}
		expectPatchStatus = func(vm, vmUpdated *azurev1alpha1.VirtualMachine) *gomock.Call {
			c.EXPECT().Get(gomock.Any(), client.ObjectKey{Namespace: namespace, Name: azureVirtualMachineName}, vm).Return(nil)
			return sw.EXPECT().Patch(gomock.Any(), vmUpdated, gomock.Any())
		}
		newVmFromState = func(state compute.ProvisioningState) *azurev1alpha1.VirtualMachine {
			if state == "" {
				return newVM(false, false, state, nil)
			} else {
				return newVM(false, true, state, nil)
			}
		}

		expectAzStateAndPromUpdate = func(azState compute.ProvisioningState) {
			switch azState {
			case compute.ProvisioningStateSucceeded:
				azureMock.ExpectGetObj(compute.ProvisioningStateSucceeded)
				promMock.ExpectSetGaugeOK()
			case compute.ProvisioningStateFailed:
				azureMock.ExpectGetObj(compute.ProvisioningStateFailed)
				promMock.ExpectSetGauge(virtualmachine.VMStateFailedWillReapply)
			case "":
				promMock.ExpectDeleteGauge()
				azureMock.EXPECT().Get(ctx, azureVirtualMachineName).Return(nil, nil)
			default:
				panic("unexpected provisioning state")
			}
		}

		expectPatchProvisioningState = func(currentState, patchedState compute.ProvisioningState) *gomock.Call {
			vm := newVmFromState(currentState)
			c.EXPECT().Get(gomock.Any(), client.ObjectKey{Namespace: namespace, Name: azureVirtualMachineName}, vm).Return(nil)
			return sw.EXPECT().Patch(gomock.Any(), newVmFromState(patchedState), gomock.Any())
		}
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#CreateOrUpdate", func() {
		It("should add the VirtualMachine object status if the VM is found", func() {
			initialStatus := compute.ProvisioningState("")
			expectAzStateAndPromUpdate(compute.ProvisioningStateSucceeded)

			expectPatchProvisioningState(initialStatus, compute.ProvisioningStateSucceeded).Return(nil)
			vm := newVmFromState(initialStatus)
			requeueAfter, err := actuator.CreateOrUpdate(ctx, vm.DeepCopyObject().(client.Object))
			Expect(err).NotTo(HaveOccurred())
			Expect(requeueAfter).To(Equal(syncPeriod))
		})

		It("should not update the VirtualMachine object status if the VM is not found", func() {
			expectAzStateAndPromUpdate("")

			initialStatus := compute.ProvisioningState("")
			vm := newVmFromState(initialStatus)
			c.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: nodeName}, vm).Return(nil)

			requeueAfter, err := actuator.CreateOrUpdate(ctx, vm.DeepCopyObject().(client.Object))
			Expect(err).NotTo(HaveOccurred())
			Expect(requeueAfter).To(Equal(requeueInterval))
		})

		It("should not update the VirtualMachine object status if the VM is found and the status is already initialized", func() {
			initialStatus := compute.ProvisioningStateSucceeded
			expectAzStateAndPromUpdate(initialStatus)

			initialVmStatus := newVmFromState(initialStatus)
			// do not patch status since up-to-date
			c.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: nodeName}, initialVmStatus).Return(nil)

			requeueAfter, err := actuator.CreateOrUpdate(ctx, initialVmStatus.DeepCopyObject().(client.Object))
			Expect(err).NotTo(HaveOccurred())
			Expect(requeueAfter).To(Equal(syncPeriod))
		})

		It("should update the VirtualMachine object status if the VM is not found and the status is already initialized", func() {
			initialStatus := compute.ProvisioningStateSucceeded
			expectAzStateAndPromUpdate("")
			expectPatchProvisioningState(initialStatus, "").Return(nil)

			vm := newVmFromState(initialStatus)
			requeueAfter, err := actuator.CreateOrUpdate(ctx, vm.DeepCopyObject().(client.Object))
			Expect(err).NotTo(HaveOccurred())
			Expect(requeueAfter).To(Equal(requeueInterval))
		})

		It("should reapply the Azure VM if it's in a failed state", func() {
			initialStatus := compute.ProvisioningState("")
			expectAzStateAndPromUpdate(compute.ProvisioningStateFailed)
			expectPatchProvisioningState(initialStatus, compute.ProvisioningStateFailed).Return(nil)

			azureMock.EXPECT().Reapply(ctx, azureVirtualMachineName).Return(nil)
			reappliedVMsCounter.EXPECT().Inc()

			expectAzStateAndPromUpdate(compute.ProvisioningStateSucceeded)
			expectPatchProvisioningState(compute.ProvisioningStateFailed, compute.ProvisioningStateSucceeded).Return(nil)

			vm := newVmFromState(initialStatus)
			requeueAfter, err := actuator.CreateOrUpdate(ctx, vm.DeepCopyObject().(client.Object))
			Expect(err).NotTo(HaveOccurred())
			Expect(requeueAfter).To(Equal(syncPeriod))
		})

		It("should fail if getting the Azure VM fails", func() {
			azureMock.EXPECT().Get(ctx, azureVirtualMachineName).Return(nil, errors.New("test"))

			initialVm := newVmFromState("")
			vmWithFailedOps := newVM(false, false, "", []azurev1alpha1.FailedOperation{
				{
					Type:         azurev1alpha1.OperationTypeGetVirtualMachine,
					Attempts:     1,
					ErrorMessage: "could not get Azure virtual machine: test",
					Timestamp:    now,
				},
			})
			expectPatchStatus(initialVm, vmWithFailedOps).Return(nil)

			_, err := actuator.CreateOrUpdate(ctx, initialVm.DeepCopyObject().(client.Object))
			Expect(err).To(BeAssignableToTypeOf(&controllererror.RequeueAfterError{}))
			re := err.(*controllererror.RequeueAfterError)
			Expect(re.Cause).To(MatchError("could not get Azure virtual machine: test"))
			Expect(re.RequeueAfter).To(Equal(requeueInterval))
		})

		It("should fail if updating the VirtualMachine object status fails", func() {
			azureMock.ExpectGetObj(compute.ProvisioningStateSucceeded)

			vm := newVM(false, false, "", nil)
			vmWithStatus := newVM(false, true, compute.ProvisioningStateSucceeded, nil)
			expectPatchStatus(vm, vmWithStatus).Return(errors.New("test"))

			_, err := actuator.CreateOrUpdate(ctx, vm.DeepCopyObject().(client.Object))
			Expect(err).To(MatchError("could not update virtualmachine status: test"))
		})

		It("should fail if reapplying the Azure VM fails", func() {
			azState := compute.ProvisioningStateFailed
			azureMock.ExpectGetObj(azState)

			vm := newVM(true, false, "", nil)
			vmWithStatus := newVM(true, true, compute.ProvisioningStateFailed, nil)
			expectPatchStatus(vm, vmWithStatus).Return(nil)

			promMock.ExpectSetGauge(virtualmachine.VMStateFailedWillReapply)
			azureMock.EXPECT().Reapply(ctx, azureVirtualMachineName).Return(errors.New("test"))

			vmWithFailedOps := newVM(true, true, compute.ProvisioningStateFailed, []azurev1alpha1.FailedOperation{
				{
					Type:         azurev1alpha1.OperationTypeReapplyVirtualMachine,
					Attempts:     1,
					ErrorMessage: "could not reapply Azure virtual machine: test",
					Timestamp:    now,
				},
			})
			expectPatchStatus(vmWithStatus, vmWithFailedOps).Return(nil)

			_, err := actuator.CreateOrUpdate(ctx, vm.DeepCopyObject().(client.Object))
			Expect(err).To(BeAssignableToTypeOf(&controllererror.RequeueAfterError{}))
			re := err.(*controllererror.RequeueAfterError)
			Expect(re.Cause).To(MatchError("could not reapply Azure virtual machine: test"))
			Expect(re.RequeueAfter).To(Equal(requeueInterval))
		})

		It("should not fail if reapplying the Azure VM fails and max attempts have been reached", func() {
			expectAzStateAndPromUpdate(compute.ProvisioningStateFailed)

			vmWithFailedOps := newVM(true, true, compute.ProvisioningStateFailed, []azurev1alpha1.FailedOperation{
				{
					Type:         azurev1alpha1.OperationTypeReapplyVirtualMachine,
					Attempts:     1,
					ErrorMessage: "could not reapply Azure virtual machine: unknown",
					Timestamp:    now,
				},
			})
			c.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: nodeName}, vmWithFailedOps).Return(nil)

			azureMock.EXPECT().Reapply(ctx, azureVirtualMachineName).Return(errors.New("test"))
			promMock.ExpectSetGauge(virtualmachine.VMStateFailed)
			vmWithFailedOps2 := newVM(true, true, compute.ProvisioningStateFailed, []azurev1alpha1.FailedOperation{
				{
					Type:         azurev1alpha1.OperationTypeReapplyVirtualMachine,
					Attempts:     2,
					ErrorMessage: "could not reapply Azure virtual machine: test",
					Timestamp:    now,
				},
			})
			expectPatchStatus(vmWithFailedOps, vmWithFailedOps2).Return(nil)

			requeueAfter, err := actuator.CreateOrUpdate(ctx, vmWithFailedOps.DeepCopyObject().(client.Object))
			Expect(err).NotTo(HaveOccurred())
			Expect(requeueAfter).To(Equal(syncPeriod))
		})

		It("should clear failed operations if reapplying the Azure VM eventually succeeds", func() {
			expectAzStateAndPromUpdate(compute.ProvisioningStateFailed)

			initialVm := newVM(true, true, compute.ProvisioningStateFailed, []azurev1alpha1.FailedOperation{
				{
					Type:         azurev1alpha1.OperationTypeReapplyVirtualMachine,
					Attempts:     1,
					ErrorMessage: "could not reapply Azure virtual machine: unknown",
					Timestamp:    now,
				},
			})
			c.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: nodeName}, initialVm).Return(nil)

			azureMock.EXPECT().Reapply(ctx, azureVirtualMachineName).Return(nil)
			reappliedVMsCounter.EXPECT().Inc()

			expectAzStateAndPromUpdate(compute.ProvisioningStateSucceeded)

			vmSucceeded := newVM(true, true, compute.ProvisioningStateSucceeded, nil)
			expectPatchStatus(initialVm, vmSucceeded).Return(nil)

			requeueAfter, err := actuator.CreateOrUpdate(ctx, initialVm.DeepCopyObject().(client.Object))
			Expect(err).NotTo(HaveOccurred())
			Expect(requeueAfter).To(Equal(syncPeriod))
		})
	})

	Describe("#Delete", func() {
		It("should update the VirtualMachine object status if the VM is found", func() {
			expectAzStateAndPromUpdate(compute.ProvisioningStateSucceeded)

			initialState := compute.ProvisioningState("")
			expectPatchProvisioningState(initialState, compute.ProvisioningStateSucceeded).Return(nil)

			vm := newVmFromState(initialState)
			requeueAfter, err := actuator.Delete(ctx, vm.DeepCopyObject().(client.Object))
			Expect(err).NotTo(HaveOccurred())
			Expect(requeueAfter).To(Equal(time.Duration(0)))
		})

		It("should not update the VirtualMachine object status if the VM is not found", func() {
			expectAzStateAndPromUpdate("")
			vm := newVM(false, false, "", nil)
			c.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: nodeName}, vm).Return(nil)

			requeueAfter, err := actuator.Delete(ctx, vm.DeepCopyObject().(client.Object))
			Expect(err).NotTo(HaveOccurred())
			Expect(requeueAfter).To(Equal(time.Duration(0)))
		})

		It("should not update the VirtualMachine object status if the VM is found and the status is already initialized", func() {
			vmWithStatus := newVM(false, true, compute.ProvisioningStateSucceeded, nil)

			expectAzStateAndPromUpdate(compute.ProvisioningStateSucceeded)
			c.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: nodeName}, vmWithStatus).Return(nil)

			requeueAfter, err := actuator.Delete(ctx, vmWithStatus.DeepCopyObject().(client.Object))
			Expect(err).NotTo(HaveOccurred())
			Expect(requeueAfter).To(Equal(time.Duration(0)))
		})

		It("should update the VirtualMachine object status if the VM is not found and the status is already initialized", func() {
			azState := compute.ProvisioningState("")
			expectAzStateAndPromUpdate(azState)

			initialState := compute.ProvisioningStateSucceeded
			expectPatchProvisioningState(initialState, azState).Return(nil)

			vmWithStatus := newVmFromState(initialState)
			requeueAfter, err := actuator.Delete(ctx, vmWithStatus.DeepCopyObject().(client.Object))
			Expect(err).NotTo(HaveOccurred())
			Expect(requeueAfter).To(Equal(time.Duration(0)))
		})

		It("should fail if getting the Azure VM fails", func() {
			azureMock.EXPECT().Get(ctx, azureVirtualMachineName).Return(nil, errors.New("test"))

			vm := newVmFromState("")
			vmWithFailedOps := newVM(false, false, "", []azurev1alpha1.FailedOperation{
				{
					Type:         azurev1alpha1.OperationTypeGetVirtualMachine,
					Attempts:     1,
					ErrorMessage: "could not get Azure virtual machine: test",
					Timestamp:    now,
				},
			})
			expectPatchStatus(vm, vmWithFailedOps).Return(nil)

			_, err := actuator.Delete(ctx, vm.DeepCopyObject().(client.Object))
			Expect(err).To(BeAssignableToTypeOf(&controllererror.RequeueAfterError{}))
			re := err.(*controllererror.RequeueAfterError)
			Expect(re.Cause).To(MatchError("could not get Azure virtual machine: test"))
			Expect(re.RequeueAfter).To(Equal(requeueInterval))
		})

		It("should fail if updating the VirtualMachine object status fails", func() {
			expectAzStateAndPromUpdate(compute.ProvisioningStateSucceeded)
			initialState := compute.ProvisioningState("")
			expectPatchProvisioningState(initialState, compute.ProvisioningStateSucceeded).Return(errors.New("test"))

			vm := newVmFromState(initialState)
			_, err := actuator.Delete(ctx, vm.DeepCopyObject().(client.Object))
			Expect(err).To(MatchError("could not update virtualmachine status: test"))
		})
	})
})
