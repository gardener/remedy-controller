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

package azure_test

import (
	"context"
	"net/http"

	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2018-11-01/network"
	"github.com/Azure/go-autorest/autorest"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
	"k8s.io/utils/pointer"

	clientazure "github.wdf.sap.corp/kubernetes/remedy-controller/pkg/client/azure"
	mockclientazure "github.wdf.sap.corp/kubernetes/remedy-controller/pkg/mock/remedy-controller/client/azure"
	"github.wdf.sap.corp/kubernetes/remedy-controller/pkg/utils/azure"
)

var _ = Describe("Azure", func() {
	const (
		ip                       = "1.2.3.4"
		resourceGroup            = "shoot--dev--test"
		azurePublicIPAddressID   = "/subscriptions/00d2caa5-cd29-46f7-845a-2f8ee0360ef5/resourceGroups/shoot--dev--test/providers/Microsoft.Network/publicIPAddresses/shoot--dev--test-a8d62a199dcbd4735bb9c90dc8b05bd1"
		azurePublicIPAddressName = "shoot--dev--test-a8d62a199dcbd4735bb9c90dc8b05bd1"
	)

	var (
		ctrl *gomock.Controller
		ctx  context.Context

		publicIPAddressesClient *mockclientazure.MockPublicIPAddressesClient
		loadBalancersClient     *mockclientazure.MockLoadBalancersClient

		pubipUtils azure.PublicIPAddressUtils

		azurePublicIPAddress network.PublicIPAddress
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		ctx = context.TODO()

		publicIPAddressesClient = mockclientazure.NewMockPublicIPAddressesClient(ctrl)
		loadBalancersClient = mockclientazure.NewMockLoadBalancersClient(ctrl)
		clients := &clientazure.Clients{
			PublicIPAddressesClient: publicIPAddressesClient,
			LoadBalancersClient:     loadBalancersClient,
		}

		pubipUtils = azure.NewPublicIPAddressUtils(clients, resourceGroup)

		azurePublicIPAddress = network.PublicIPAddress{
			ID:   pointer.StringPtr(azurePublicIPAddressID),
			Name: pointer.StringPtr(azurePublicIPAddressName),
			PublicIPAddressPropertiesFormat: &network.PublicIPAddressPropertiesFormat{
				IPAddress:         pointer.StringPtr(ip),
				ProvisioningState: pointer.StringPtr(string(network.Succeeded)),
			},
		}
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#GetByName", func() {
		It("should return the Azure PublicIPAddress if it is found", func() {
			publicIPAddressesClient.EXPECT().Get(ctx, resourceGroup, azurePublicIPAddressName, "").Return(azurePublicIPAddress, nil)

			result, err := pubipUtils.GetByName(ctx, azurePublicIPAddressName)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(&azurePublicIPAddress))
		})

		It("should return nil if the Azure PublicIPAddress is not found", func() {
			publicIPAddressesClient.EXPECT().Get(ctx, resourceGroup, azurePublicIPAddressName, "").
				Return(network.PublicIPAddress{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: http.StatusNotFound}, ""))

			result, err := pubipUtils.GetByName(ctx, azurePublicIPAddressName)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(BeNil())
		})

		It("should fail if getting the Azure PublicIPAddress fails", func() {
			publicIPAddressesClient.EXPECT().Get(ctx, resourceGroup, azurePublicIPAddressName, "").Return(network.PublicIPAddress{}, errors.New("test"))

			_, err := pubipUtils.GetByName(ctx, azurePublicIPAddressName)
			Expect(err).To(MatchError("could not get Azure PublicIPAddress: test"))
		})
	})

	Describe("#GetByIP", func() {
		It("should return the Azure PublicIPAddress if it is found", func() {
			page := network.NewPublicIPAddressListResultPage(func(context.Context, network.PublicIPAddressListResult) (network.PublicIPAddressListResult, error) {
				return network.PublicIPAddressListResult{
					Value: &[]network.PublicIPAddress{azurePublicIPAddress},
				}, nil
			})
			Expect(page.NextWithContext(ctx)).To(Succeed())
			publicIPAddressesClient.EXPECT().List(ctx, resourceGroup).Return(page, nil)

			result, err := pubipUtils.GetByIP(ctx, ip)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(&azurePublicIPAddress))
		})

		It("should return nil if the Azure PublicIPAddress is not found", func() {
			page := network.NewPublicIPAddressListResultPage(func(context.Context, network.PublicIPAddressListResult) (network.PublicIPAddressListResult, error) {
				return network.PublicIPAddressListResult{
					Value: &[]network.PublicIPAddress{},
				}, nil
			})
			Expect(page.NextWithContext(ctx)).To(Succeed())
			publicIPAddressesClient.EXPECT().List(ctx, resourceGroup).Return(page, nil)

			result, err := pubipUtils.GetByIP(ctx, ip)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(BeNil())
		})

		It("should fail if listing Azure PublicIPAddresses fails", func() {
			publicIPAddressesClient.EXPECT().List(ctx, resourceGroup).Return(network.PublicIPAddressListResultPage{}, errors.New("test"))

			_, err := pubipUtils.GetByIP(ctx, ip)
			Expect(err).To(MatchError("could not list Azure PublicIPAddresses: test"))
		})
	})
})
