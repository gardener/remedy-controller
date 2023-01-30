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

package azure_test

import (
	"context"
	"net/http"

	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2019-07-01/compute"
	"github.com/Azure/go-autorest/autorest"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
	"k8s.io/utils/pointer"

	clientazure "github.com/gardener/remedy-controller/pkg/client/azure"
	mockprometheus "github.com/gardener/remedy-controller/pkg/mock/prometheus"
	mockclientazure "github.com/gardener/remedy-controller/pkg/mock/remedy-controller/client/azure"
	"github.com/gardener/remedy-controller/pkg/utils/azure"
)

var _ = Describe("VirtualMachineUtils", func() {
	const (
		resourceGroup      = "shoot--dev--test"
		virtualMachineID   = "/subscriptions/xxx/resourceGroups/shoot--dev--test/providers/Microsoft.Compute/virtualMachines/shoot--dev--test-vm1"
		virtualMachineName = "shoot--dev--test-vm1"
	)

	var (
		ctrl *gomock.Controller
		ctx  context.Context

		vmClient             *mockclientazure.MockVirtualMachinesClient
		future               *mockclientazure.MockFuture
		readRequestsCounter  *mockprometheus.MockCounter
		writeRequestsCounter *mockprometheus.MockCounter

		vmUtils azure.VirtualMachineUtils

		virtualMachine compute.VirtualMachine

		notFoundError error
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		ctx = context.TODO()

		vmClient = mockclientazure.NewMockVirtualMachinesClient(ctrl)
		future = mockclientazure.NewMockFuture(ctrl)
		readRequestsCounter = mockprometheus.NewMockCounter(ctrl)
		writeRequestsCounter = mockprometheus.NewMockCounter(ctrl)
		clients := &clientazure.Clients{
			VirtualMachinesClient: vmClient,
		}

		vmUtils = azure.NewVirtualMachineUtils(clients, resourceGroup, readRequestsCounter, writeRequestsCounter)

		virtualMachine = compute.VirtualMachine{
			ID:                       pointer.StringPtr(virtualMachineID),
			Name:                     pointer.StringPtr(virtualMachineName),
			VirtualMachineProperties: &compute.VirtualMachineProperties{},
		}

		notFoundError = autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: http.StatusNotFound}, "")
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#Get", func() {
		It("should return the Azure VirtualMachine if it is found", func() {
			vmClient.EXPECT().Get(ctx, resourceGroup, virtualMachineName, compute.InstanceView).Return(virtualMachine, nil)
			readRequestsCounter.EXPECT().Inc()

			result, err := vmUtils.Get(ctx, virtualMachineName)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(&virtualMachine))
		})

		It("should return nil if the Azure VirtualMachine is not found", func() {
			vmClient.EXPECT().Get(ctx, resourceGroup, virtualMachineName, compute.InstanceView).Return(compute.VirtualMachine{}, notFoundError)
			readRequestsCounter.EXPECT().Inc()

			result, err := vmUtils.Get(ctx, virtualMachineName)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(BeNil())
		})

		It("should fail if getting the Azure VirtualMachine fails", func() {
			vmClient.EXPECT().Get(ctx, resourceGroup, virtualMachineName, compute.InstanceView).Return(compute.VirtualMachine{}, errors.New("test"))
			readRequestsCounter.EXPECT().Inc()

			_, err := vmUtils.Get(ctx, virtualMachineName)
			Expect(err).To(MatchError("could not get Azure VirtualMachine: test"))
		})
	})

	Describe("#Reapply", func() {
		It("should reapply the Azure VirtualMachine if it is found", func() {
			vmClient.EXPECT().Reapply(ctx, resourceGroup, virtualMachineName).Return(future, nil)
			vmClient.EXPECT().Client().Return(autorest.Client{})
			future.EXPECT().WaitForCompletionRef(ctx, autorest.Client{}).Return(nil)
			readRequestsCounter.EXPECT().Inc()
			writeRequestsCounter.EXPECT().Inc()

			Expect(vmUtils.Reapply(ctx, virtualMachineName)).To(Succeed())
		})

		It("should fail if reapplying the Azure VirtualMachine fails", func() {
			vmClient.EXPECT().Reapply(ctx, resourceGroup, virtualMachineName).Return(future, errors.New("test"))
			writeRequestsCounter.EXPECT().Inc()

			err := vmUtils.Reapply(ctx, virtualMachineName)
			Expect(err).To(MatchError("could not reapply Azure VirtualMachine: test"))
		})

		It("should fail if waiting for the Azure VirtualMachine reapply to complete fails", func() {
			vmClient.EXPECT().Reapply(ctx, resourceGroup, virtualMachineName).Return(future, nil)
			vmClient.EXPECT().Client().Return(autorest.Client{})
			future.EXPECT().WaitForCompletionRef(ctx, autorest.Client{}).Return(errors.New("test"))
			readRequestsCounter.EXPECT().Inc()
			writeRequestsCounter.EXPECT().Inc()

			err := vmUtils.Reapply(ctx, virtualMachineName)
			Expect(err).To(MatchError("could not wait for the Azure VirtualMachine reapply to complete: test"))
		})
	})
})
