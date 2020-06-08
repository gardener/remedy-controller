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

package publicipaddress_test

import (
	"context"
	"net/http"
	"time"

	azureinstall "github.wdf.sap.corp/kubernetes/remedy-controller/pkg/apis/azure/install"
	azurev1alpha1 "github.wdf.sap.corp/kubernetes/remedy-controller/pkg/apis/azure/v1alpha1"
	"github.wdf.sap.corp/kubernetes/remedy-controller/pkg/apis/config"
	"github.wdf.sap.corp/kubernetes/remedy-controller/pkg/client/azure"
	"github.wdf.sap.corp/kubernetes/remedy-controller/pkg/controller"
	"github.wdf.sap.corp/kubernetes/remedy-controller/pkg/controller/azure/publicipaddress"
	mockclient "github.wdf.sap.corp/kubernetes/remedy-controller/pkg/mock/controller-runtime/client"
	mockclientazure "github.wdf.sap.corp/kubernetes/remedy-controller/pkg/mock/remedy-controller/client/azure"

	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2018-11-01/network"
	"github.com/Azure/go-autorest/autorest"
	"github.com/go-logr/logr"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/runtime/inject"
)

var _ = Describe("Actuator", func() {
	const (
		serviceName              = "test-service"
		namespace                = "test"
		ip                       = "1.2.3.4"
		resourceGroup            = "shoot--dev--test"
		azurePublicIPAddressID   = "/subscriptions/00d2caa5-cd29-46f7-845a-2f8ee0360ef5/resourceGroups/shoot--dev--test/providers/Microsoft.Network/publicIPAddresses/shoot--dev--test-a8d62a199dcbd4735bb9c90dc8b05bd1"
		azurePublicIPAddressName = "shoot--dev--test-a8d62a199dcbd4735bb9c90dc8b05bd1"
	)

	var (
		ctrl *gomock.Controller
		ctx  context.Context

		scheme *runtime.Scheme

		c                       *mockclient.MockClient
		sw                      *mockclient.MockStatusWriter
		publicIPAddressesClient *mockclientazure.MockPublicIPAddressesClient
		loadBalancersClient     *mockclientazure.MockLoadBalancersClient

		cfg      config.AzurePublicIPRemedyConfiguration
		logger   logr.Logger
		actuator controller.Actuator

		pubip                *azurev1alpha1.PublicIPAddress
		pubipWithStatus      *azurev1alpha1.PublicIPAddress
		azurePublicIPAddress network.PublicIPAddress
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		ctx = context.TODO()

		scheme = runtime.NewScheme()
		Expect(azureinstall.AddToScheme(scheme)).To(Succeed())

		c = mockclient.NewMockClient(ctrl)
		sw = mockclient.NewMockStatusWriter(ctrl)
		c.EXPECT().Status().Return(sw).AnyTimes()
		publicIPAddressesClient = mockclientazure.NewMockPublicIPAddressesClient(ctrl)
		loadBalancersClient = mockclientazure.NewMockLoadBalancersClient(ctrl)
		clients := &azure.Clients{
			PublicIPAddressesClient: publicIPAddressesClient,
			LoadBalancersClient:     loadBalancersClient,
		}

		cfg = config.AzurePublicIPRemedyConfiguration{
			RequeueInterval:     metav1.Duration{Duration: 1 * time.Second},
			DeletionGracePeriod: metav1.Duration{Duration: 1 * time.Second},
		}
		logger = log.Log.WithName("test")
		actuator = publicipaddress.NewActuator(clients, resourceGroup, cfg, logger)
		Expect(actuator.(inject.Client).InjectClient(c)).To(Succeed())

		pubip = &azurev1alpha1.PublicIPAddress{
			ObjectMeta: metav1.ObjectMeta{
				Name:      serviceName + "-" + ip,
				Namespace: namespace,
			},
			Spec: azurev1alpha1.PublicIPAddressSpec{
				IPAddress: ip,
			},
		}
		pubipWithStatus = &azurev1alpha1.PublicIPAddress{
			ObjectMeta: metav1.ObjectMeta{
				Name:      serviceName + "-" + ip,
				Namespace: namespace,
			},
			Spec: azurev1alpha1.PublicIPAddressSpec{
				IPAddress: ip,
			},
			Status: azurev1alpha1.PublicIPAddressStatus{
				Exists:            true,
				ID:                pointer.StringPtr(azurePublicIPAddressID),
				Name:              pointer.StringPtr(azurePublicIPAddressName),
				ProvisioningState: pointer.StringPtr(string(network.Succeeded)),
			},
		}

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

	Describe("#CreateOrUpdate", func() {
		It("should update the PublicIPAddress object status if the IP is found", func() {
			page := network.NewPublicIPAddressListResultPage(func(context.Context, network.PublicIPAddressListResult) (network.PublicIPAddressListResult, error) {
				return network.PublicIPAddressListResult{
					Value: &[]network.PublicIPAddress{azurePublicIPAddress},
				}, nil
			})
			Expect(page.NextWithContext(ctx)).To(Succeed())
			publicIPAddressesClient.EXPECT().List(ctx, resourceGroup).Return(page, nil)
			c.EXPECT().Get(ctx, client.ObjectKey{Namespace: pubip.Namespace, Name: pubip.Name}, pubip).Return(nil)
			sw.EXPECT().Update(ctx, pubipWithStatus).Return(nil)

			requeueAfter, removeFinalizer, err := actuator.CreateOrUpdate(ctx, pubip)
			Expect(err).NotTo(HaveOccurred())
			Expect(requeueAfter).To(Equal(time.Duration(0)))
			Expect(removeFinalizer).To(Equal(false))
		})

		It("should not update the PublicIPAddress object status if the IP is not found", func() {
			page := network.NewPublicIPAddressListResultPage(func(context.Context, network.PublicIPAddressListResult) (network.PublicIPAddressListResult, error) {
				return network.PublicIPAddressListResult{
					Value: &[]network.PublicIPAddress{},
				}, nil
			})
			Expect(page.NextWithContext(ctx)).To(Succeed())
			publicIPAddressesClient.EXPECT().List(ctx, resourceGroup).Return(page, nil)
			c.EXPECT().Get(ctx, client.ObjectKey{Namespace: pubip.Namespace, Name: pubip.Name}, pubip).Return(nil)

			requeueAfter, removeFinalizer, err := actuator.CreateOrUpdate(ctx, pubip)
			Expect(err).NotTo(HaveOccurred())
			Expect(requeueAfter).To(Equal(1 * time.Second))
			Expect(removeFinalizer).To(Equal(false))
		})

		It("should not update the PublicIPAddress object status if the IP is found and the status is already up-to-date", func() {
			publicIPAddressesClient.EXPECT().Get(ctx, resourceGroup, azurePublicIPAddressName, "").Return(azurePublicIPAddress, nil)
			c.EXPECT().Get(ctx, client.ObjectKey{Namespace: pubipWithStatus.Namespace, Name: pubipWithStatus.Name}, pubipWithStatus).Return(nil)

			requeueAfter, removeFinalizer, err := actuator.CreateOrUpdate(ctx, pubipWithStatus)
			Expect(err).NotTo(HaveOccurred())
			Expect(requeueAfter).To(Equal(time.Duration(0)))
			Expect(removeFinalizer).To(Equal(false))
		})

		It("should fail if listing IP addresses fails", func() {
			publicIPAddressesClient.EXPECT().List(ctx, resourceGroup).Return(network.PublicIPAddressListResultPage{}, errors.New("test"))

			_, _, err := actuator.CreateOrUpdate(ctx, pubip)
			Expect(err).To(MatchError("could not list Azure public IP addresses: test"))
		})

		It("should update the PublicIPAddress object status if the IP is not found and the status is already initialized", func() {
			publicIPAddressesClient.EXPECT().Get(ctx, resourceGroup, azurePublicIPAddressName, "").Return(network.PublicIPAddress{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: http.StatusNotFound}, ""))
			c.EXPECT().Get(ctx, client.ObjectKey{Namespace: pubipWithStatus.Namespace, Name: pubipWithStatus.Name}, pubipWithStatus).Return(nil)
			sw.EXPECT().Update(ctx, pubip).Return(nil)

			requeueAfter, removeFinalizer, err := actuator.CreateOrUpdate(ctx, pubipWithStatus)
			Expect(err).NotTo(HaveOccurred())
			Expect(requeueAfter).To(Equal(1 * time.Second))
			Expect(removeFinalizer).To(Equal(false))
		})
	})
})
