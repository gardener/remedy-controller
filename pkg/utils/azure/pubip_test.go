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

	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2018-11-01/network"
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

var _ = Describe("PublicIPAddressUtils", func() {
	const (
		ip                           = "1.2.3.4"
		ip2                          = "5.6.7.8"
		resourceGroup                = "shoot--dev--test"
		publicIPAddressID            = "/subscriptions/xxx/resourceGroups/shoot--dev--test/providers/Microsoft.Network/publicIPAddresses/shoot--dev--test-ip1"
		publicIPAddressName          = "shoot--dev--test-ip1"
		publicIPAddressID2           = "/subscriptions/xxx/resourceGroups/shoot--dev--test/providers/Microsoft.Network/publicIPAddresses/shoot--dev--test-ip2"
		publicIPAddressName2         = "shoot--dev--test-ip2"
		frontendIPConfigurationID    = "/subscriptions/xxx/resourceGroups/shoot--dev--test/providers/Microsoft.Network/loadBalancers/shoot--dev--test/frontendIPConfigurations/ip1"
		frontendIPConfigurationName  = "ip1"
		frontendIPConfigurationID2   = "/subscriptions/xxx/resourceGroups/shoot--dev--test/providers/Microsoft.Network/loadBalancers/shoot--dev--test/frontendIPConfigurations/ip2"
		frontendIPConfigurationName2 = "ip2"
		loadBalancingRuleID          = "/subscriptions/xxx/resourceGroups/shoot--dev--test/providers/Microsoft.Network/loadBalancers/shoot--dev--test/loadBalancingRules/ip1-UDP-1234"
		loadBalancingRuleName        = "ip1-UDP-1234"
		loadBalancingRuleID2         = "/subscriptions/xxx/resourceGroups/shoot--dev--test/providers/Microsoft.Network/loadBalancers/shoot--dev--test/loadBalancingRules/ip2-UDP-1234"
		loadBalancingRuleName2       = "ip2-UDP-1234"
		probeID                      = "/subscriptions/xxx/resourceGroups/shoot--dev--test/providers/Microsoft.Network/loadBalancers/shoot--dev--test/probes/ip1-TCP-4314"
		probeName                    = "ip1-TCP-4314"
		probeID2                     = "/subscriptions/xxx/resourceGroups/shoot--dev--test/providers/Microsoft.Network/loadBalancers/shoot--dev--test/probes/ip2-TCP-4314"
		probeName2                   = "ip2-TCP-4314"
		loadBalancerID               = "/subscriptions/xxx/resourceGroups/shoot--dev--test/providers/Microsoft.Network/loadBalancers/shoot--dev--test"
		loadBalancerName             = "shoot--dev--test"
	)

	var (
		ctrl *gomock.Controller
		ctx  context.Context

		publicIPAddressesClient *mockclientazure.MockPublicIPAddressesClient
		loadBalancersClient     *mockclientazure.MockLoadBalancersClient
		future                  *mockclientazure.MockFuture
		readRequestsCounter     *mockprometheus.MockCounter
		writeRequestsCounter    *mockprometheus.MockCounter

		pubipUtils azure.PublicIPAddressUtils

		publicIPAddress          network.PublicIPAddress
		publicIPAddress2         network.PublicIPAddress
		frontendIPConfiguration  network.FrontendIPConfiguration
		frontendIPConfiguration2 network.FrontendIPConfiguration
		loadBalancingRule        network.LoadBalancingRule
		loadBalancingRule2       network.LoadBalancingRule
		probe                    network.Probe
		probe2                   network.Probe

		newLoadBalancer func([]network.FrontendIPConfiguration, []network.LoadBalancingRule, []network.Probe) network.LoadBalancer

		newPublicIPAddressListResultPage func([]network.PublicIPAddress, bool) network.PublicIPAddressListResultPage

		notFoundError error
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		ctx = context.TODO()

		publicIPAddressesClient = mockclientazure.NewMockPublicIPAddressesClient(ctrl)
		loadBalancersClient = mockclientazure.NewMockLoadBalancersClient(ctrl)
		future = mockclientazure.NewMockFuture(ctrl)
		readRequestsCounter = mockprometheus.NewMockCounter(ctrl)
		writeRequestsCounter = mockprometheus.NewMockCounter(ctrl)
		clients := &clientazure.Clients{
			PublicIPAddressesClient: publicIPAddressesClient,
			LoadBalancersClient:     loadBalancersClient,
		}

		pubipUtils = azure.NewPublicIPAddressUtils(clients, resourceGroup, readRequestsCounter, writeRequestsCounter)

		publicIPAddress = network.PublicIPAddress{
			ID:   pointer.StringPtr(publicIPAddressID),
			Name: pointer.StringPtr(publicIPAddressName),
			PublicIPAddressPropertiesFormat: &network.PublicIPAddressPropertiesFormat{
				IPAddress: pointer.StringPtr(ip),
			},
		}
		publicIPAddress2 = network.PublicIPAddress{
			ID:   pointer.StringPtr(publicIPAddressID2),
			Name: pointer.StringPtr(publicIPAddressName2),
			PublicIPAddressPropertiesFormat: &network.PublicIPAddressPropertiesFormat{
				IPAddress: pointer.StringPtr(ip2),
			},
		}
		frontendIPConfiguration = network.FrontendIPConfiguration{
			ID:   pointer.StringPtr(frontendIPConfigurationID),
			Name: pointer.StringPtr(frontendIPConfigurationName),
			FrontendIPConfigurationPropertiesFormat: &network.FrontendIPConfigurationPropertiesFormat{
				LoadBalancingRules: &[]network.SubResource{
					{ID: pointer.StringPtr(loadBalancingRuleID)},
				},
				PublicIPAddress: &network.PublicIPAddress{
					ID: pointer.StringPtr(publicIPAddressID),
				},
			},
		}
		frontendIPConfiguration2 = network.FrontendIPConfiguration{
			ID:   pointer.StringPtr(frontendIPConfigurationID2),
			Name: pointer.StringPtr(frontendIPConfigurationName2),
			FrontendIPConfigurationPropertiesFormat: &network.FrontendIPConfigurationPropertiesFormat{
				LoadBalancingRules: &[]network.SubResource{
					{ID: pointer.StringPtr(loadBalancingRuleID2)},
				},
				PublicIPAddress: &network.PublicIPAddress{
					ID: pointer.StringPtr(publicIPAddressID2),
				},
			},
		}
		loadBalancingRule = network.LoadBalancingRule{
			ID:   pointer.StringPtr(loadBalancingRuleID),
			Name: pointer.StringPtr(loadBalancingRuleName),
			LoadBalancingRulePropertiesFormat: &network.LoadBalancingRulePropertiesFormat{
				FrontendIPConfiguration: &network.SubResource{ID: pointer.StringPtr(frontendIPConfigurationID)},
			},
		}
		loadBalancingRule2 = network.LoadBalancingRule{
			ID:   pointer.StringPtr(loadBalancingRuleID2),
			Name: pointer.StringPtr(loadBalancingRuleName2),
			LoadBalancingRulePropertiesFormat: &network.LoadBalancingRulePropertiesFormat{
				FrontendIPConfiguration: &network.SubResource{ID: pointer.StringPtr(frontendIPConfigurationID2)},
			},
		}
		probe = network.Probe{
			ID:   pointer.StringPtr(probeID),
			Name: pointer.StringPtr(probeName),
			ProbePropertiesFormat: &network.ProbePropertiesFormat{
				LoadBalancingRules: &[]network.SubResource{
					{ID: pointer.StringPtr(loadBalancingRuleID)},
				},
			},
		}
		probe2 = network.Probe{
			ID:   pointer.StringPtr(probeID2),
			Name: pointer.StringPtr(probeName2),
			ProbePropertiesFormat: &network.ProbePropertiesFormat{
				LoadBalancingRules: &[]network.SubResource{
					{ID: pointer.StringPtr(loadBalancingRuleID2)},
				},
			},
		}

		newLoadBalancer = func(frontendIPConfigurations []network.FrontendIPConfiguration, loadBalancingRules []network.LoadBalancingRule, probes []network.Probe) network.LoadBalancer {
			return network.LoadBalancer{
				ID:   pointer.StringPtr(loadBalancerID),
				Name: pointer.StringPtr(loadBalancerName),
				LoadBalancerPropertiesFormat: &network.LoadBalancerPropertiesFormat{
					FrontendIPConfigurations: &frontendIPConfigurations,
					LoadBalancingRules:       &loadBalancingRules,
					Probes:                   &probes,
				},
			}
		}

		newPublicIPAddressListResultPage = func(publicIPAddresses []network.PublicIPAddress, fail bool) network.PublicIPAddressListResultPage {
			page := network.NewPublicIPAddressListResultPage(network.PublicIPAddressListResult{}, func(_ context.Context, res network.PublicIPAddressListResult) (network.PublicIPAddressListResult, error) {
				if res.Value == nil {
					return network.PublicIPAddressListResult{
						Value: &publicIPAddresses,
					}, nil
				}
				if fail {
					return network.PublicIPAddressListResult{}, errors.New("test")
				}
				return network.PublicIPAddressListResult{}, nil
			})
			Expect(page.NextWithContext(ctx)).To(Succeed())
			return page
		}

		notFoundError = autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: http.StatusNotFound}, "")
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#GetByName", func() {
		It("should return the Azure PublicIPAddress if it is found", func() {
			publicIPAddressesClient.EXPECT().Get(ctx, resourceGroup, publicIPAddressName, "").Return(publicIPAddress, nil)
			readRequestsCounter.EXPECT().Inc()

			result, err := pubipUtils.GetByName(ctx, publicIPAddressName)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(&publicIPAddress))
		})

		It("should return nil if the Azure PublicIPAddress is not found", func() {
			publicIPAddressesClient.EXPECT().Get(ctx, resourceGroup, publicIPAddressName, "").Return(network.PublicIPAddress{}, notFoundError)
			readRequestsCounter.EXPECT().Inc()

			result, err := pubipUtils.GetByName(ctx, publicIPAddressName)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(BeNil())
		})

		It("should fail if getting the Azure PublicIPAddress fails", func() {
			publicIPAddressesClient.EXPECT().Get(ctx, resourceGroup, publicIPAddressName, "").Return(network.PublicIPAddress{}, errors.New("test"))
			readRequestsCounter.EXPECT().Inc()

			_, err := pubipUtils.GetByName(ctx, publicIPAddressName)
			Expect(err).To(MatchError("could not get Azure PublicIPAddress: test"))
		})
	})

	Describe("#GetByIP", func() {
		It("should return the Azure PublicIPAddress if it is found", func() {
			page := newPublicIPAddressListResultPage([]network.PublicIPAddress{publicIPAddress, publicIPAddress2}, false)
			publicIPAddressesClient.EXPECT().List(ctx, resourceGroup).Return(page, nil)
			readRequestsCounter.EXPECT().Inc()

			result, err := pubipUtils.GetByIP(ctx, ip)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(&publicIPAddress))
		})

		It("should return nil if the Azure PublicIPAddress is not found", func() {
			page := newPublicIPAddressListResultPage([]network.PublicIPAddress{publicIPAddress2}, false)
			publicIPAddressesClient.EXPECT().List(ctx, resourceGroup).Return(page, nil)
			readRequestsCounter.EXPECT().Inc().Times(2)

			result, err := pubipUtils.GetByIP(ctx, ip)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(BeNil())
		})

		It("should fail if listing Azure PublicIPAddresses fails", func() {
			publicIPAddressesClient.EXPECT().List(ctx, resourceGroup).Return(network.PublicIPAddressListResultPage{}, errors.New("test"))
			readRequestsCounter.EXPECT().Inc()

			_, err := pubipUtils.GetByIP(ctx, ip)
			Expect(err).To(MatchError("could not list Azure PublicIPAddresses: test"))
		})

		It("should fail if advancing to the next page of Azure PublicIPAddresses fails", func() {
			page := newPublicIPAddressListResultPage([]network.PublicIPAddress{publicIPAddress2}, true)
			publicIPAddressesClient.EXPECT().List(ctx, resourceGroup).Return(page, nil)
			readRequestsCounter.EXPECT().Inc().Times(2)

			_, err := pubipUtils.GetByIP(ctx, ip)
			Expect(err).To(MatchError("could not advance to the next page of Azure PublicIPAddresses: test"))
		})
	})

	Describe("#GetAll", func() {
		It("should return all Azure PublicIPAddresses", func() {
			azurePublicIPAddresses := []network.PublicIPAddress{publicIPAddress, publicIPAddress2}
			page := newPublicIPAddressListResultPage(azurePublicIPAddresses, false)
			publicIPAddressesClient.EXPECT().List(ctx, resourceGroup).Return(page, nil)
			readRequestsCounter.EXPECT().Inc().Times(2)

			result, err := pubipUtils.GetAll(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(azurePublicIPAddresses))
		})

		It("should fail if listing Azure PublicIPAddresses fails", func() {
			publicIPAddressesClient.EXPECT().List(ctx, resourceGroup).Return(network.PublicIPAddressListResultPage{}, errors.New("test"))
			readRequestsCounter.EXPECT().Inc()

			_, err := pubipUtils.GetAll(ctx)
			Expect(err).To(MatchError("could not list Azure PublicIPAddresses: test"))
		})

		It("should fail if advancing to the next page of Azure PublicIPAddresses fails", func() {
			page := newPublicIPAddressListResultPage([]network.PublicIPAddress{publicIPAddress}, true)
			publicIPAddressesClient.EXPECT().List(ctx, resourceGroup).Return(page, nil)
			readRequestsCounter.EXPECT().Inc().Times(2)

			_, err := pubipUtils.GetAll(ctx)
			Expect(err).To(MatchError("could not advance to the next page of Azure PublicIPAddresses: test"))
		})
	})

	Describe("#RemoveFromLoadBalancer", func() {
		It("should remove all obsolete resources from the Azure LoadBalancer", func() {
			loadBalancersClient.EXPECT().Get(ctx, resourceGroup, loadBalancerName, "").Return(newLoadBalancer(
				[]network.FrontendIPConfiguration{frontendIPConfiguration, frontendIPConfiguration2},
				[]network.LoadBalancingRule{loadBalancingRule, loadBalancingRule2},
				[]network.Probe{probe, probe2},
			), nil)
			loadBalancersClient.EXPECT().CreateOrUpdate(ctx, resourceGroup, loadBalancerName, newLoadBalancer(
				[]network.FrontendIPConfiguration{frontendIPConfiguration2},
				[]network.LoadBalancingRule{loadBalancingRule2},
				[]network.Probe{probe2},
			)).Return(future, nil)
			loadBalancersClient.EXPECT().Client().Return(autorest.Client{})
			future.EXPECT().WaitForCompletionRef(ctx, autorest.Client{}).Return(nil)
			readRequestsCounter.EXPECT().Inc().Times(2)
			writeRequestsCounter.EXPECT().Inc()

			Expect(pubipUtils.RemoveFromLoadBalancer(ctx, []string{publicIPAddressID})).To(Succeed())
		})

		It("should fail if getting the Azure LoadBalancer fails", func() {
			loadBalancersClient.EXPECT().Get(ctx, resourceGroup, loadBalancerName, "").Return(network.LoadBalancer{}, errors.New("test"))
			readRequestsCounter.EXPECT().Inc()

			err := pubipUtils.RemoveFromLoadBalancer(ctx, []string{publicIPAddressID})
			Expect(err).To(MatchError("could not get Azure LoadBalancer: test"))
		})

		It("should fail if updating the Azure LoadBalancer fails", func() {
			loadBalancersClient.EXPECT().Get(ctx, resourceGroup, loadBalancerName, "").Return(newLoadBalancer(nil, nil, nil), nil)
			loadBalancersClient.EXPECT().CreateOrUpdate(ctx, resourceGroup, loadBalancerName, newLoadBalancer(nil, nil, nil)).Return(future, errors.New("test"))
			readRequestsCounter.EXPECT().Inc()
			writeRequestsCounter.EXPECT().Inc()

			err := pubipUtils.RemoveFromLoadBalancer(ctx, []string{publicIPAddressID})
			Expect(err).To(MatchError("could not update Azure LoadBalancer: test"))
		})

		It("should fail if waiting for the Azure LoadBalancer update to complete fails", func() {
			loadBalancersClient.EXPECT().Get(ctx, resourceGroup, loadBalancerName, "").Return(newLoadBalancer(nil, nil, nil), nil)
			loadBalancersClient.EXPECT().CreateOrUpdate(ctx, resourceGroup, loadBalancerName, newLoadBalancer(nil, nil, nil)).Return(future, nil)
			loadBalancersClient.EXPECT().Client().Return(autorest.Client{})
			future.EXPECT().WaitForCompletionRef(ctx, autorest.Client{}).Return(errors.New("test"))
			readRequestsCounter.EXPECT().Inc().Times(2)
			writeRequestsCounter.EXPECT().Inc()

			err := pubipUtils.RemoveFromLoadBalancer(ctx, []string{publicIPAddressID})
			Expect(err).To(MatchError("could not wait for the Azure LoadBalancer update to complete: test"))
		})
	})

	Describe("#Delete", func() {
		It("should delete the Azure PublicIPAddress if it is found", func() {
			publicIPAddressesClient.EXPECT().Delete(ctx, resourceGroup, publicIPAddressName).Return(future, nil)
			publicIPAddressesClient.EXPECT().Client().Return(autorest.Client{})
			future.EXPECT().WaitForCompletionRef(ctx, autorest.Client{}).Return(nil)
			readRequestsCounter.EXPECT().Inc()
			writeRequestsCounter.EXPECT().Inc()

			Expect(pubipUtils.Delete(ctx, publicIPAddressName)).To(Succeed())
		})

		It("should not fail if the Azure PublicIPAddress is not found", func() {
			publicIPAddressesClient.EXPECT().Delete(ctx, resourceGroup, publicIPAddressName).Return(future, notFoundError)
			writeRequestsCounter.EXPECT().Inc()

			Expect(pubipUtils.Delete(ctx, publicIPAddressName)).To(Succeed())
		})

		It("should fail if deleting the Azure PublicIPAddress fails", func() {
			publicIPAddressesClient.EXPECT().Delete(ctx, resourceGroup, publicIPAddressName).Return(future, errors.New("test"))
			writeRequestsCounter.EXPECT().Inc()

			err := pubipUtils.Delete(ctx, publicIPAddressName)
			Expect(err).To(MatchError("could not delete Azure PublicIPAddress: test"))
		})

		It("should fail if waiting for the Azure PublicIPAddress deletion to complete fails", func() {
			publicIPAddressesClient.EXPECT().Delete(ctx, resourceGroup, publicIPAddressName).Return(future, nil)
			publicIPAddressesClient.EXPECT().Client().Return(autorest.Client{})
			future.EXPECT().WaitForCompletionRef(ctx, autorest.Client{}).Return(errors.New("test"))
			readRequestsCounter.EXPECT().Inc()
			writeRequestsCounter.EXPECT().Inc()

			err := pubipUtils.Delete(ctx, publicIPAddressName)
			Expect(err).To(MatchError("could not wait for the Azure PublicIPAddress deletion to complete: test"))
		})
	})
})
