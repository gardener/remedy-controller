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
		ctrl *gomock.Controller
		ctx  context.Context

		c                   *mockclient.MockClient
		sw                  *mockclient.MockStatusWriter
		vmUtils             *mockutilsazure.MockVirtualMachineUtils
		reappliedVMsCounter *mockprometheus.MockCounter
		vmStatesGaugeVec    *mockutilsprometheus.MockGaugeVec
		vmStatesGauge       *mockprometheus.MockGauge

		cfg         config.AzureFailedVMRemedyConfiguration
		now         metav1.Time
		timestamper utils.Timestamper
		logger      logr.Logger
		actuator    controller.Actuator

		newVM                  func(bool, bool, compute.ProvisioningState, []azurev1alpha1.FailedOperation) *azurev1alpha1.VirtualMachine
		newAzureVirtualMachine func(compute.ProvisioningState) *compute.VirtualMachine
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		ctx = context.TODO()

		c = mockclient.NewMockClient(ctrl)
		sw = mockclient.NewMockStatusWriter(ctrl)
		c.EXPECT().Status().Return(sw).AnyTimes()
		vmUtils = mockutilsazure.NewMockVirtualMachineUtils(ctrl)
		reappliedVMsCounter = mockprometheus.NewMockCounter(ctrl)
		vmStatesGaugeVec = mockutilsprometheus.NewMockGaugeVec(ctrl)
		vmStatesGauge = mockprometheus.NewMockGauge(ctrl)

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
		newAzureVirtualMachine = func(provisioningState compute.ProvisioningState) *compute.VirtualMachine {
			return &compute.VirtualMachine{
				ID:   pointer.StringPtr(azureVirtualMachineID),
				Name: pointer.StringPtr(azureVirtualMachineName),
				VirtualMachineProperties: &compute.VirtualMachineProperties{
					ProvisioningState: pointer.StringPtr(string(provisioningState)),
				},
			}
		}
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#CreateOrUpdate", func() {
		It("should update the VirtualMachine object status if the VM is found", func() {
			vm := newVM(false, false, "", nil)
			vmWithStatus := newVM(false, true, compute.ProvisioningStateSucceeded, nil)
			azureVirtualMachine := newAzureVirtualMachine(compute.ProvisioningStateSucceeded)
			vmUtils.EXPECT().Get(ctx, azureVirtualMachineName).Return(azureVirtualMachine, nil)
			vmStatesGaugeVec.EXPECT().WithLabelValues(azureVirtualMachineName).Return(vmStatesGauge)
			vmStatesGauge.EXPECT().Set(virtualmachine.VMStateOK)
			c.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: nodeName}, vm).Return(nil)
			sw.EXPECT().Update(ctx, vmWithStatus).Return(nil)

			requeueAfter, err := actuator.CreateOrUpdate(ctx, vm.DeepCopyObject().(client.Object))
			Expect(err).NotTo(HaveOccurred())
			Expect(requeueAfter).To(Equal(syncPeriod))
		})

		It("should not update the VirtualMachine object status if the VM is not found", func() {
			vm := newVM(false, false, "", nil)
			vmUtils.EXPECT().Get(ctx, azureVirtualMachineName).Return(nil, nil)
			vmStatesGaugeVec.EXPECT().DeleteLabelValues(azureVirtualMachineName)
			c.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: nodeName}, vm).Return(nil)

			requeueAfter, err := actuator.CreateOrUpdate(ctx, vm.DeepCopyObject().(client.Object))
			Expect(err).NotTo(HaveOccurred())
			Expect(requeueAfter).To(Equal(requeueInterval))
		})

		It("should not update the VirtualMachine object status if the VM is found and the status is already initialized", func() {
			vmWithStatus := newVM(false, true, compute.ProvisioningStateSucceeded, nil)
			azureVirtualMachine := newAzureVirtualMachine(compute.ProvisioningStateSucceeded)
			vmUtils.EXPECT().Get(ctx, azureVirtualMachineName).Return(azureVirtualMachine, nil)
			c.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: nodeName}, vmWithStatus).Return(nil)
			vmStatesGaugeVec.EXPECT().WithLabelValues(azureVirtualMachineName).Return(vmStatesGauge)
			vmStatesGauge.EXPECT().Set(virtualmachine.VMStateOK)

			requeueAfter, err := actuator.CreateOrUpdate(ctx, vmWithStatus.DeepCopyObject().(client.Object))
			Expect(err).NotTo(HaveOccurred())
			Expect(requeueAfter).To(Equal(syncPeriod))
		})

		It("should update the VirtualMachine object status if the VM is not found and the status is already initialized", func() {
			vm := newVM(false, false, "", nil)
			vmWithStatus := newVM(false, true, compute.ProvisioningStateSucceeded, nil)
			vmUtils.EXPECT().Get(ctx, azureVirtualMachineName).Return(nil, nil)
			vmStatesGaugeVec.EXPECT().DeleteLabelValues(azureVirtualMachineName)
			c.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: nodeName}, vmWithStatus).Return(nil)
			sw.EXPECT().Update(ctx, vm).Return(nil)

			requeueAfter, err := actuator.CreateOrUpdate(ctx, vmWithStatus.DeepCopyObject().(client.Object))
			Expect(err).NotTo(HaveOccurred())
			Expect(requeueAfter).To(Equal(requeueInterval))
		})

		It("should reapply the Azure VM if it's in a failed state", func() {
			vm := newVM(true, false, "", nil)
			vmWithStatus := newVM(true, true, compute.ProvisioningStateFailed, nil)
			vmWithStatus2 := newVM(true, true, compute.ProvisioningStateSucceeded, nil)
			azureVirtualMachine := newAzureVirtualMachine(compute.ProvisioningStateFailed)
			azureVirtualMachine2 := newAzureVirtualMachine(compute.ProvisioningStateSucceeded)
			vmUtils.EXPECT().Get(ctx, azureVirtualMachineName).Return(azureVirtualMachine, nil)
			c.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: nodeName}, vm).Return(nil)
			sw.EXPECT().Update(ctx, vmWithStatus).Return(nil)
			vmStatesGaugeVec.EXPECT().WithLabelValues(azureVirtualMachineName).Return(vmStatesGauge)
			vmStatesGauge.EXPECT().Set(virtualmachine.VMStateFailedWillReapply)
			vmUtils.EXPECT().Reapply(ctx, azureVirtualMachineName).Return(nil)
			vmUtils.EXPECT().Get(ctx, azureVirtualMachineName).Return(azureVirtualMachine2, nil)
			reappliedVMsCounter.EXPECT().Inc()
			vmStatesGaugeVec.EXPECT().WithLabelValues(azureVirtualMachineName).Return(vmStatesGauge)
			vmStatesGauge.EXPECT().Set(virtualmachine.VMStateOK)
			c.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: nodeName}, vmWithStatus).Return(nil)
			sw.EXPECT().Update(ctx, vmWithStatus2).Return(nil)

			requeueAfter, err := actuator.CreateOrUpdate(ctx, vm.DeepCopyObject().(client.Object))
			Expect(err).NotTo(HaveOccurred())
			Expect(requeueAfter).To(Equal(syncPeriod))
		})

		It("should fail if getting the Azure VM fails", func() {
			vm := newVM(false, false, "", nil)
			vmWithFailedOps := newVM(false, false, "", []azurev1alpha1.FailedOperation{
				{
					Type:         azurev1alpha1.OperationTypeGetVirtualMachine,
					Attempts:     1,
					ErrorMessage: "could not get Azure virtual machine: test",
					Timestamp:    now,
				},
			})
			vmUtils.EXPECT().Get(ctx, azureVirtualMachineName).Return(nil, errors.New("test"))
			c.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: nodeName}, vm).Return(nil)
			sw.EXPECT().Update(ctx, vmWithFailedOps).Return(nil)

			_, err := actuator.CreateOrUpdate(ctx, vm.DeepCopyObject().(client.Object))
			Expect(err).To(BeAssignableToTypeOf(&controllererror.RequeueAfterError{}))
			re := err.(*controllererror.RequeueAfterError)
			Expect(re.Cause).To(MatchError("could not get Azure virtual machine: test"))
			Expect(re.RequeueAfter).To(Equal(requeueInterval))
		})

		It("should fail if updating the VirtualMachine object status fails", func() {
			vm := newVM(false, false, "", nil)
			vmWithStatus := newVM(false, true, compute.ProvisioningStateSucceeded, nil)
			azureVirtualMachine := newAzureVirtualMachine(compute.ProvisioningStateSucceeded)
			vmUtils.EXPECT().Get(ctx, azureVirtualMachineName).Return(azureVirtualMachine, nil)
			c.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: nodeName}, vm).Return(nil)
			sw.EXPECT().Update(ctx, vmWithStatus).Return(errors.New("test"))

			_, err := actuator.CreateOrUpdate(ctx, vm.DeepCopyObject().(client.Object))
			Expect(err).To(MatchError("could not update virtualmachine status: test"))
		})

		It("should fail if reapplying the Azure VM fails", func() {
			vm := newVM(true, false, "", nil)
			vmWithStatus := newVM(true, true, compute.ProvisioningStateFailed, nil)
			vmWithFailedOps := newVM(true, true, compute.ProvisioningStateFailed, []azurev1alpha1.FailedOperation{
				{
					Type:         azurev1alpha1.OperationTypeReapplyVirtualMachine,
					Attempts:     1,
					ErrorMessage: "could not reapply Azure virtual machine: test",
					Timestamp:    now,
				},
			})
			azureVirtualMachine := newAzureVirtualMachine(compute.ProvisioningStateFailed)
			vmUtils.EXPECT().Get(ctx, azureVirtualMachineName).Return(azureVirtualMachine, nil)
			c.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: nodeName}, vm).Return(nil)
			sw.EXPECT().Update(ctx, vmWithStatus).Return(nil)
			vmStatesGaugeVec.EXPECT().WithLabelValues(azureVirtualMachineName).Return(vmStatesGauge)
			vmStatesGauge.EXPECT().Set(virtualmachine.VMStateFailedWillReapply)
			vmUtils.EXPECT().Reapply(ctx, azureVirtualMachineName).Return(errors.New("test"))
			c.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: nodeName}, vmWithStatus).Return(nil)
			sw.EXPECT().Update(ctx, vmWithFailedOps).Return(nil)

			_, err := actuator.CreateOrUpdate(ctx, vm.DeepCopyObject().(client.Object))
			Expect(err).To(BeAssignableToTypeOf(&controllererror.RequeueAfterError{}))
			re := err.(*controllererror.RequeueAfterError)
			Expect(re.Cause).To(MatchError("could not reapply Azure virtual machine: test"))
			Expect(re.RequeueAfter).To(Equal(requeueInterval))
		})

		It("should not fail if reapplying the Azure VM fails and max attempts have been reached", func() {
			vmWithFailedOps := newVM(true, true, compute.ProvisioningStateFailed, []azurev1alpha1.FailedOperation{
				{
					Type:         azurev1alpha1.OperationTypeReapplyVirtualMachine,
					Attempts:     1,
					ErrorMessage: "could not reapply Azure virtual machine: unknown",
					Timestamp:    now,
				},
			})
			vmWithFailedOps2 := newVM(true, true, compute.ProvisioningStateFailed, []azurev1alpha1.FailedOperation{
				{
					Type:         azurev1alpha1.OperationTypeReapplyVirtualMachine,
					Attempts:     2,
					ErrorMessage: "could not reapply Azure virtual machine: test",
					Timestamp:    now,
				},
			})
			azureVirtualMachine := newAzureVirtualMachine(compute.ProvisioningStateFailed)
			vmUtils.EXPECT().Get(ctx, azureVirtualMachineName).Return(azureVirtualMachine, nil)
			c.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: nodeName}, vmWithFailedOps).Return(nil)
			vmStatesGaugeVec.EXPECT().WithLabelValues(azureVirtualMachineName).Return(vmStatesGauge)
			vmStatesGauge.EXPECT().Set(virtualmachine.VMStateFailedWillReapply)
			vmUtils.EXPECT().Reapply(ctx, azureVirtualMachineName).Return(errors.New("test"))
			vmStatesGaugeVec.EXPECT().WithLabelValues(azureVirtualMachineName).Return(vmStatesGauge)
			vmStatesGauge.EXPECT().Set(virtualmachine.VMStateFailed)
			c.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: nodeName}, vmWithFailedOps).Return(nil)
			sw.EXPECT().Update(ctx, vmWithFailedOps2).Return(nil)

			requeueAfter, err := actuator.CreateOrUpdate(ctx, vmWithFailedOps.DeepCopyObject().(client.Object))
			Expect(err).NotTo(HaveOccurred())
			Expect(requeueAfter).To(Equal(syncPeriod))
		})

		It("should clear failed operations if reapplying the Azure VM eventually succeeds", func() {
			vmWithFailedOps := newVM(true, true, compute.ProvisioningStateFailed, []azurev1alpha1.FailedOperation{
				{
					Type:         azurev1alpha1.OperationTypeReapplyVirtualMachine,
					Attempts:     1,
					ErrorMessage: "could not reapply Azure virtual machine: unknown",
					Timestamp:    now,
				},
			})
			vm := newVM(true, true, compute.ProvisioningStateSucceeded, nil)
			azureVirtualMachine := newAzureVirtualMachine(compute.ProvisioningStateFailed)
			azureVirtualMachine2 := newAzureVirtualMachine(compute.ProvisioningStateSucceeded)
			vmUtils.EXPECT().Get(ctx, azureVirtualMachineName).Return(azureVirtualMachine, nil)
			c.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: nodeName}, vmWithFailedOps).Return(nil)
			vmStatesGaugeVec.EXPECT().WithLabelValues(azureVirtualMachineName).Return(vmStatesGauge)
			vmStatesGauge.EXPECT().Set(virtualmachine.VMStateFailedWillReapply)
			vmUtils.EXPECT().Reapply(ctx, azureVirtualMachineName).Return(nil)
			vmUtils.EXPECT().Get(ctx, azureVirtualMachineName).Return(azureVirtualMachine2, nil)
			reappliedVMsCounter.EXPECT().Inc()
			vmStatesGaugeVec.EXPECT().WithLabelValues(azureVirtualMachineName).Return(vmStatesGauge)
			vmStatesGauge.EXPECT().Set(virtualmachine.VMStateOK)
			c.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: nodeName}, vmWithFailedOps).Return(nil)
			sw.EXPECT().Update(ctx, vm).Return(nil)

			requeueAfter, err := actuator.CreateOrUpdate(ctx, vmWithFailedOps.DeepCopyObject().(client.Object))
			Expect(err).NotTo(HaveOccurred())
			Expect(requeueAfter).To(Equal(syncPeriod))
		})
	})

	Describe("#Delete", func() {
		It("should update the VirtualMachine object status if the VM is found", func() {
			vm := newVM(false, false, "", nil)
			vmWithStatus := newVM(false, true, compute.ProvisioningStateSucceeded, nil)
			azureVirtualMachine := newAzureVirtualMachine(compute.ProvisioningStateSucceeded)
			vmUtils.EXPECT().Get(ctx, azureVirtualMachineName).Return(azureVirtualMachine, nil)
			vmStatesGaugeVec.EXPECT().WithLabelValues(azureVirtualMachineName).Return(vmStatesGauge)
			vmStatesGauge.EXPECT().Set(virtualmachine.VMStateOK)
			c.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: nodeName}, vm).Return(nil)
			sw.EXPECT().Update(ctx, vmWithStatus).Return(nil)

			requeueAfter, err := actuator.Delete(ctx, vm.DeepCopyObject().(client.Object))
			Expect(err).NotTo(HaveOccurred())
			Expect(requeueAfter).To(Equal(time.Duration(0)))
		})

		It("should not update the VirtualMachine object status if the VM is not found", func() {
			vm := newVM(false, false, "", nil)
			vmUtils.EXPECT().Get(ctx, azureVirtualMachineName).Return(nil, nil)
			vmStatesGaugeVec.EXPECT().DeleteLabelValues(azureVirtualMachineName)
			c.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: nodeName}, vm).Return(nil)

			requeueAfter, err := actuator.Delete(ctx, vm.DeepCopyObject().(client.Object))
			Expect(err).NotTo(HaveOccurred())
			Expect(requeueAfter).To(Equal(time.Duration(0)))
		})

		It("should not update the VirtualMachine object status if the VM is found and the status is already initialized", func() {
			vmWithStatus := newVM(false, true, compute.ProvisioningStateSucceeded, nil)
			azureVirtualMachine := newAzureVirtualMachine(compute.ProvisioningStateSucceeded)
			vmUtils.EXPECT().Get(ctx, azureVirtualMachineName).Return(azureVirtualMachine, nil)
			vmStatesGaugeVec.EXPECT().WithLabelValues(azureVirtualMachineName).Return(vmStatesGauge)
			vmStatesGauge.EXPECT().Set(virtualmachine.VMStateOK)
			c.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: nodeName}, vmWithStatus).Return(nil)

			requeueAfter, err := actuator.Delete(ctx, vmWithStatus.DeepCopyObject().(client.Object))
			Expect(err).NotTo(HaveOccurred())
			Expect(requeueAfter).To(Equal(time.Duration(0)))
		})

		It("should update the VirtualMachine object status if the VM is not found and the status is already initialized", func() {
			vm := newVM(false, false, "", nil)
			vmWithStatus := newVM(false, true, compute.ProvisioningStateSucceeded, nil)
			vmUtils.EXPECT().Get(ctx, azureVirtualMachineName).Return(nil, nil)
			vmStatesGaugeVec.EXPECT().DeleteLabelValues(azureVirtualMachineName)
			c.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: nodeName}, vmWithStatus).Return(nil)
			sw.EXPECT().Update(ctx, vm).Return(nil)

			requeueAfter, err := actuator.Delete(ctx, vmWithStatus.DeepCopyObject().(client.Object))
			Expect(err).NotTo(HaveOccurred())
			Expect(requeueAfter).To(Equal(time.Duration(0)))
		})

		It("should fail if getting the Azure VM fails", func() {
			vm := newVM(false, false, "", nil)
			vmWithFailedOps := newVM(false, false, "", []azurev1alpha1.FailedOperation{
				{
					Type:         azurev1alpha1.OperationTypeGetVirtualMachine,
					Attempts:     1,
					ErrorMessage: "could not get Azure virtual machine: test",
					Timestamp:    now,
				},
			})
			vmUtils.EXPECT().Get(ctx, azureVirtualMachineName).Return(nil, errors.New("test"))
			c.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: nodeName}, vm).Return(nil)
			sw.EXPECT().Update(ctx, vmWithFailedOps).Return(nil)

			_, err := actuator.Delete(ctx, vm.DeepCopyObject().(client.Object))
			Expect(err).To(BeAssignableToTypeOf(&controllererror.RequeueAfterError{}))
			re := err.(*controllererror.RequeueAfterError)
			Expect(re.Cause).To(MatchError("could not get Azure virtual machine: test"))
			Expect(re.RequeueAfter).To(Equal(requeueInterval))
		})

		It("should fail if updating the VirtualMachine object status fails", func() {
			vm := newVM(false, false, "", nil)
			vmWithStatus := newVM(false, true, compute.ProvisioningStateSucceeded, nil)
			azureVirtualMachine := newAzureVirtualMachine(compute.ProvisioningStateSucceeded)
			vmUtils.EXPECT().Get(ctx, azureVirtualMachineName).Return(azureVirtualMachine, nil)
			vmStatesGaugeVec.EXPECT().WithLabelValues(azureVirtualMachineName).Return(vmStatesGauge)
			vmStatesGauge.EXPECT().Set(virtualmachine.VMStateOK)
			c.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: nodeName}, vm).Return(nil)
			sw.EXPECT().Update(ctx, vmWithStatus).Return(errors.New("test"))

			_, err := actuator.Delete(ctx, vm.DeepCopyObject().(client.Object))
			Expect(err).To(MatchError("could not update virtualmachine status: test"))
		})
	})
})
