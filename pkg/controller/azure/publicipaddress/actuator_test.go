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

package publicipaddress_test

import (
	"context"
	"strconv"
	"time"

	azurev1alpha1 "github.com/gardener/remedy-controller/pkg/apis/azure/v1alpha1"
	"github.com/gardener/remedy-controller/pkg/apis/config"
	"github.com/gardener/remedy-controller/pkg/controller"
	"github.com/gardener/remedy-controller/pkg/controller/azure"
	"github.com/gardener/remedy-controller/pkg/controller/azure/publicipaddress"
	mockclient "github.com/gardener/remedy-controller/pkg/mock/controller-runtime/client"
	mockprometheus "github.com/gardener/remedy-controller/pkg/mock/prometheus"
	mockutilsazure "github.com/gardener/remedy-controller/pkg/mock/remedy-controller/utils/azure"
	"github.com/gardener/remedy-controller/pkg/utils"

	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2018-11-01/network"
	controllererror "github.com/gardener/gardener/pkg/controllerutils/reconciler"
	"github.com/go-logr/logr"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
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
		serviceName              = "test-service"
		namespace                = "test"
		ip                       = "1.2.3.4"
		ip2                      = "5.6.7.8"
		pubipName                = serviceName + "-" + ip
		azurePublicIPAddressID   = "/subscriptions/xxx/resourceGroups/shoot--dev--test/providers/Microsoft.Network/publicIPAddresses/shoot--dev--test-ip1"
		azurePublicIPAddressName = "shoot--dev--test-ip1"

		requeueInterval     = 1 * time.Second
		syncPeriod          = 1 * time.Minute
		deletionGracePeriod = 1 * time.Second
	)

	var (
		ctrl *gomock.Controller
		ctx  context.Context

		c                 *mockclient.MockClient
		sw                *mockclient.MockStatusWriter
		pubipUtils        *mockutilsazure.MockPublicIPAddressUtils
		cleanedIPsCounter *mockprometheus.MockCounter

		cfg         config.AzureOrphanedPublicIPRemedyConfiguration
		now         metav1.Time
		timestamper utils.Timestamper
		logger      logr.Logger
		actuator    controller.Actuator

		earlyDeletionTimestamp metav1.Time

		newPubip                func(bool, []azurev1alpha1.FailedOperation, *metav1.Time, map[string]string) *azurev1alpha1.PublicIPAddress
		newFailedOps            func(azurev1alpha1.OperationType, int, string) []azurev1alpha1.FailedOperation
		newAzurePublicIPAddress func(string, bool) *network.PublicIPAddress
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		ctx = context.TODO()

		c = mockclient.NewMockClient(ctrl)
		sw = mockclient.NewMockStatusWriter(ctrl)
		c.EXPECT().Status().Return(sw).AnyTimes()
		pubipUtils = mockutilsazure.NewMockPublicIPAddressUtils(ctrl)
		cleanedIPsCounter = mockprometheus.NewMockCounter(ctrl)

		cfg = config.AzureOrphanedPublicIPRemedyConfiguration{
			RequeueInterval:     metav1.Duration{Duration: requeueInterval},
			SyncPeriod:          metav1.Duration{Duration: syncPeriod},
			DeletionGracePeriod: metav1.Duration{Duration: deletionGracePeriod},
			MaxGetAttempts:      2,
			MaxCleanAttempts:    2,
		}
		now = metav1.Now()
		timestamper = utils.TimestamperFunc(func() metav1.Time { return now })
		logger = log.Log.WithName("test")
		actuator = publicipaddress.NewActuator(pubipUtils, cfg, timestamper, logger, cleanedIPsCounter)
		Expect(actuator.(inject.Client).InjectClient(c)).To(Succeed())

		earlyDeletionTimestamp = metav1.NewTime(now.Add(-10 * time.Minute))

		newPubip = func(withStatus bool, failedOperations []azurev1alpha1.FailedOperation, deletionTimestamp *metav1.Time, annotations map[string]string) *azurev1alpha1.PublicIPAddress {
			var status azurev1alpha1.PublicIPAddressStatus
			if withStatus {
				status = azurev1alpha1.PublicIPAddressStatus{
					Exists:            true,
					ID:                pointer.StringPtr(azurePublicIPAddressID),
					Name:              pointer.StringPtr(azurePublicIPAddressName),
					ProvisioningState: pointer.StringPtr(string(network.Succeeded)),
				}
			}
			status.FailedOperations = failedOperations
			return &azurev1alpha1.PublicIPAddress{
				ObjectMeta: metav1.ObjectMeta{
					Name:              pubipName,
					Namespace:         namespace,
					DeletionTimestamp: deletionTimestamp,
					Labels: map[string]string{
						azure.ServiceLabel: namespace + "." + serviceName,
					},
					Annotations: annotations,
				},
				Spec: azurev1alpha1.PublicIPAddressSpec{
					IPAddress: ip,
				},
				Status: status,
			}
		}
		newFailedOps = func(opType azurev1alpha1.OperationType, attempts int, errorMessage string) []azurev1alpha1.FailedOperation {
			return []azurev1alpha1.FailedOperation{
				{
					Type:         opType,
					Attempts:     attempts,
					ErrorMessage: errorMessage,
					Timestamp:    now,
				},
			}
		}
		newAzurePublicIPAddress = func(ip string, withServiceTag bool) *network.PublicIPAddress {
			var tags map[string]*string
			if withServiceTag {
				tags = map[string]*string{
					publicipaddress.ServiceTag: pointer.StringPtr(namespace + "/" + serviceName),
				}
			}
			return &network.PublicIPAddress{
				ID:   pointer.StringPtr(azurePublicIPAddressID),
				Name: pointer.StringPtr(azurePublicIPAddressName),
				PublicIPAddressPropertiesFormat: &network.PublicIPAddressPropertiesFormat{
					IPAddress:         pointer.StringPtr(ip),
					ProvisioningState: pointer.StringPtr(string(network.Succeeded)),
				},
				Tags: tags,
			}
		}
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#CreateOrUpdate", func() {
		It("should update the PublicIPAddress object status if the IP is found", func() {
			pubip, pubipWithStatus := newPubip(false, nil, nil, nil), newPubip(true, nil, nil, nil)
			azurePublicIPAddress := newAzurePublicIPAddress(ip, true)
			pubipUtils.EXPECT().GetByIP(ctx, ip).Return(azurePublicIPAddress, nil)
			c.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: pubipName}, pubip).Return(nil)
			sw.EXPECT().Update(ctx, pubipWithStatus).Return(nil)

			requeueAfter, err := actuator.CreateOrUpdate(ctx, pubip.DeepCopyObject().(client.Object))
			Expect(err).NotTo(HaveOccurred())
			Expect(requeueAfter).To(Equal(syncPeriod))
		})

		It("should not update the PublicIPAddress object status if the IP is not found", func() {
			pubip := newPubip(false, nil, nil, nil)
			pubipUtils.EXPECT().GetByIP(ctx, ip).Return(nil, nil)
			c.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: pubipName}, pubip).Return(nil)

			requeueAfter, err := actuator.CreateOrUpdate(ctx, pubip.DeepCopyObject().(client.Object))
			Expect(err).NotTo(HaveOccurred())
			Expect(requeueAfter).To(Equal(requeueInterval))
		})

		It("should not update the PublicIPAddress object status if the IP is found but doesn't have the service tag", func() {
			pubip := newPubip(false, nil, nil, nil)
			azurePublicIPAddress := newAzurePublicIPAddress(ip, false)
			pubipUtils.EXPECT().GetByIP(ctx, ip).Return(azurePublicIPAddress, nil)
			c.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: pubipName}, pubip).Return(nil)

			requeueAfter, err := actuator.CreateOrUpdate(ctx, pubip.DeepCopyObject().(client.Object))
			Expect(err).NotTo(HaveOccurred())
			Expect(requeueAfter).To(Equal(requeueInterval))
		})

		It("should not update the PublicIPAddress object status if the IP is found and the status is already initialized", func() {
			pubipWithStatus := newPubip(true, nil, nil, nil)
			azurePublicIPAddress := newAzurePublicIPAddress(ip, true)
			pubipUtils.EXPECT().GetByName(ctx, azurePublicIPAddressName).Return(azurePublicIPAddress, nil)
			c.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: pubipName}, pubipWithStatus).Return(nil)

			requeueAfter, err := actuator.CreateOrUpdate(ctx, pubipWithStatus.DeepCopyObject().(client.Object))
			Expect(err).NotTo(HaveOccurred())
			Expect(requeueAfter).To(Equal(syncPeriod))
		})

		It("should update the PublicIPAddress object status if the IP is not found and the status is already initialized", func() {
			pubip, pubipWithStatus := newPubip(false, nil, nil, nil), newPubip(true, nil, nil, nil)
			pubipUtils.EXPECT().GetByName(ctx, azurePublicIPAddressName).Return(nil, nil)
			pubipUtils.EXPECT().GetByIP(ctx, ip).Return(nil, nil)
			c.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: pubipName}, pubipWithStatus).Return(nil)
			sw.EXPECT().Update(ctx, pubip).Return(nil)

			requeueAfter, err := actuator.CreateOrUpdate(ctx, pubipWithStatus.DeepCopyObject().(client.Object))
			Expect(err).NotTo(HaveOccurred())
			Expect(requeueAfter).To(Equal(requeueInterval))
		})

		It("should update the PublicIPAddress object status if the IP address has changed and the status is already initialized", func() {
			pubip, pubipWithStatus := newPubip(false, nil, nil, nil), newPubip(true, nil, nil, nil)
			azurePublicIPAddress2 := newAzurePublicIPAddress(ip2, true)
			pubipUtils.EXPECT().GetByName(ctx, azurePublicIPAddressName).Return(azurePublicIPAddress2, nil)
			pubipUtils.EXPECT().GetByIP(ctx, ip).Return(nil, nil)
			c.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: pubipName}, pubipWithStatus).Return(nil)
			sw.EXPECT().Update(ctx, pubip).Return(nil)

			requeueAfter, err := actuator.CreateOrUpdate(ctx, pubipWithStatus.DeepCopyObject().(client.Object))
			Expect(err).NotTo(HaveOccurred())
			Expect(requeueAfter).To(Equal(requeueInterval))
		})

		It("should fail and requeue if getting the Azure IP address by IP fails", func() {
			pubip := newPubip(false, nil, nil, nil)
			failedOps := newFailedOps(azurev1alpha1.OperationTypeGetPublicIPAddress, 1, "could not get Azure public IP address by IP: test")
			pubipWithFailedOps := newPubip(false, failedOps, nil, nil)
			pubipUtils.EXPECT().GetByIP(ctx, ip).Return(nil, errors.New("test"))
			c.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: pubipName}, pubip).Return(nil)
			sw.EXPECT().Update(ctx, pubipWithFailedOps).Return(nil)

			_, err := actuator.CreateOrUpdate(ctx, pubip.DeepCopyObject().(client.Object))
			Expect(err).To(BeAssignableToTypeOf(&controllererror.RequeueAfterError{}))
			re := err.(*controllererror.RequeueAfterError)
			Expect(re.Cause).To(MatchError("could not get Azure public IP address by IP: test"))
			Expect(re.RequeueAfter).To(Equal(requeueInterval))
		})

		It("should not fail if getting the Azure IP address by IP fails and max attempts have been reached", func() {
			failedOps := newFailedOps(azurev1alpha1.OperationTypeGetPublicIPAddress, cfg.MaxGetAttempts-1, "could not get Azure public IP address by IP: test")
			pubip := newPubip(false, failedOps, nil, nil)
			failedOps2 := newFailedOps(azurev1alpha1.OperationTypeGetPublicIPAddress, cfg.MaxGetAttempts, "could not get Azure public IP address by IP: test")
			pubip2 := newPubip(false, failedOps2, nil, nil)
			pubipUtils.EXPECT().GetByIP(ctx, ip).Return(nil, errors.New("test"))
			c.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: pubipName}, pubip).Return(nil)
			sw.EXPECT().Update(ctx, pubip2).Return(nil)

			requeueAfter, err := actuator.CreateOrUpdate(ctx, pubip.DeepCopyObject().(client.Object))
			Expect(err).NotTo(HaveOccurred())
			Expect(requeueAfter).To(Equal(syncPeriod))
		})

		It("should fail if updating the PublicIPAddress object status fails", func() {
			pubip, pubipWithStatus := newPubip(false, nil, nil, nil), newPubip(true, nil, nil, nil)
			azurePublicIPAddress := newAzurePublicIPAddress(ip, true)
			pubipUtils.EXPECT().GetByIP(ctx, ip).Return(azurePublicIPAddress, nil)
			c.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: pubipName}, pubip).Return(nil)
			sw.EXPECT().Update(ctx, pubipWithStatus).Return(errors.New("test"))

			_, err := actuator.CreateOrUpdate(ctx, pubip.DeepCopyObject().(client.Object))
			Expect(err).To(MatchError("could not update publicipaddress status: test"))
		})
	})

	Describe("#Delete", func() {
		It("should clean the IP and update the PublicIPAddress object status if the IP is found", func() {
			pubip := newPubip(false, nil, &earlyDeletionTimestamp, nil)
			pubipWithStatus := newPubip(true, nil, &earlyDeletionTimestamp, nil)
			azurePublicIPAddress := newAzurePublicIPAddress(ip, true)
			pubipUtils.EXPECT().GetByIP(ctx, ip).Return(azurePublicIPAddress, nil)
			c.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: pubipName}, pubip).Return(nil)
			sw.EXPECT().Update(ctx, pubipWithStatus).Return(nil)
			pubipUtils.EXPECT().RemoveFromLoadBalancer(ctx, []string{string(azurePublicIPAddressID)}).Return(nil)
			pubipUtils.EXPECT().Delete(ctx, azurePublicIPAddressName).Return(nil)
			cleanedIPsCounter.EXPECT().Inc()
			c.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: pubipName}, pubipWithStatus).Return(nil)
			sw.EXPECT().Update(ctx, pubip).Return(nil)

			requeueAfter, err := actuator.Delete(ctx, pubip.DeepCopyObject().(client.Object))
			Expect(err).NotTo(HaveOccurred())
			Expect(requeueAfter).To(Equal(time.Duration(0)))
		})

		It("should not update the PublicIPAddress object status if the IP is not found", func() {
			pubip := newPubip(false, nil, &earlyDeletionTimestamp, nil)
			pubipUtils.EXPECT().GetByIP(ctx, ip).Return(nil, nil)
			c.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: pubipName}, pubip).Return(nil)

			requeueAfter, err := actuator.Delete(ctx, pubip.DeepCopyObject().(client.Object))
			Expect(err).NotTo(HaveOccurred())
			Expect(requeueAfter).To(Equal(time.Duration(0)))
		})

		It("should not update the PublicIPAddress object status if the IP is found but doesn't have the service tag", func() {
			pubip := newPubip(false, nil, &earlyDeletionTimestamp, nil)
			azurePublicIPAddress := newAzurePublicIPAddress(ip, false)
			pubipUtils.EXPECT().GetByIP(ctx, ip).Return(azurePublicIPAddress, nil)
			c.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: pubipName}, pubip).Return(nil)

			requeueAfter, err := actuator.Delete(ctx, pubip.DeepCopyObject().(client.Object))
			Expect(err).NotTo(HaveOccurred())
			Expect(requeueAfter).To(Equal(time.Duration(0)))
		})

		It("should clean the IP and not update the PublicIPAddress object status if the IP is found and the status is already initialized", func() {
			pubip := newPubip(false, nil, &earlyDeletionTimestamp, nil)
			pubipWithStatus := newPubip(true, nil, &earlyDeletionTimestamp, nil)
			azurePublicIPAddress := newAzurePublicIPAddress(ip, true)
			pubipUtils.EXPECT().GetByName(ctx, azurePublicIPAddressName).Return(azurePublicIPAddress, nil)
			c.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: pubipName}, pubipWithStatus).Return(nil)
			pubipUtils.EXPECT().RemoveFromLoadBalancer(ctx, []string{string(azurePublicIPAddressID)}).Return(nil)
			pubipUtils.EXPECT().Delete(ctx, azurePublicIPAddressName).Return(nil)
			cleanedIPsCounter.EXPECT().Inc()
			c.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: pubipName}, pubipWithStatus).Return(nil)
			sw.EXPECT().Update(ctx, pubip).Return(nil)

			requeueAfter, err := actuator.Delete(ctx, pubipWithStatus.DeepCopyObject().(client.Object))
			Expect(err).NotTo(HaveOccurred())
			Expect(requeueAfter).To(Equal(time.Duration(0)))
		})

		It("should update the PublicIPAddress object status if the IP is not found and the status is already initialized", func() {
			pubip := newPubip(false, nil, &earlyDeletionTimestamp, nil)
			pubipWithStatus := newPubip(true, nil, &earlyDeletionTimestamp, nil)
			pubipUtils.EXPECT().GetByName(ctx, azurePublicIPAddressName).Return(nil, nil)
			pubipUtils.EXPECT().GetByIP(ctx, ip).Return(nil, nil)
			c.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: pubipName}, pubipWithStatus).Return(nil)
			sw.EXPECT().Update(ctx, pubip).Return(nil)

			requeueAfter, err := actuator.Delete(ctx, pubipWithStatus.DeepCopyObject().(client.Object))
			Expect(err).NotTo(HaveOccurred())
			Expect(requeueAfter).To(Equal(time.Duration(0)))
		})

		It("should update the PublicIPAddress object status if the IP address has changed and the status is already initialized", func() {
			pubip := newPubip(false, nil, &earlyDeletionTimestamp, nil)
			pubipWithStatus := newPubip(true, nil, &earlyDeletionTimestamp, nil)
			azurePublicIPAddress2 := newAzurePublicIPAddress(ip2, true)
			pubipUtils.EXPECT().GetByName(ctx, azurePublicIPAddressName).Return(azurePublicIPAddress2, nil)
			pubipUtils.EXPECT().GetByIP(ctx, ip).Return(nil, nil)
			c.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: pubipName}, pubipWithStatus).Return(nil)
			sw.EXPECT().Update(ctx, pubip).Return(nil)

			requeueAfter, err := actuator.Delete(ctx, pubipWithStatus.DeepCopyObject().(client.Object))
			Expect(err).NotTo(HaveOccurred())
			Expect(requeueAfter).To(Equal(time.Duration(0)))
		})

		It("should not clean the IP if it has the do-not-clean annotation", func() {
			annotations := map[string]string{azure.DoNotCleanAnnotation: strconv.FormatBool(true)}
			pubip := newPubip(false, nil, &earlyDeletionTimestamp, annotations)
			pubipWithStatus := newPubip(true, nil, &earlyDeletionTimestamp, annotations)
			azurePublicIPAddress := newAzurePublicIPAddress(ip, true)
			pubipUtils.EXPECT().GetByIP(ctx, ip).Return(azurePublicIPAddress, nil)
			c.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: pubipName}, pubip).Return(nil)
			sw.EXPECT().Update(ctx, pubipWithStatus).Return(nil)

			requeueAfter, err := actuator.Delete(ctx, pubip.DeepCopyObject().(client.Object))
			Expect(err).NotTo(HaveOccurred())
			Expect(requeueAfter).To(Equal(time.Duration(0)))
		})

		It("should fail if updating the PublicIPAddress object status fails", func() {
			pubip := newPubip(false, nil, &earlyDeletionTimestamp, nil)
			pubipWithStatus := newPubip(true, nil, &earlyDeletionTimestamp, nil)
			azurePublicIPAddress := newAzurePublicIPAddress(ip, true)
			pubipUtils.EXPECT().GetByIP(ctx, ip).Return(azurePublicIPAddress, nil)
			c.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: pubipName}, pubip).Return(nil)
			sw.EXPECT().Update(ctx, pubipWithStatus).Return(errors.New("test"))

			_, err := actuator.Delete(ctx, pubip.DeepCopyObject().(client.Object))
			Expect(err).To(MatchError("could not update publicipaddress status: test"))
		})

		It("should honour the grace period before cleaning the IP", func() {
			pubip := newPubip(true, nil, &now, nil)
			azurePublicIPAddress := newAzurePublicIPAddress(ip, true)
			pubipUtils.EXPECT().GetByName(ctx, azurePublicIPAddressName).Return(azurePublicIPAddress, nil)
			c.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: pubipName}, pubip).Return(nil)

			_, err := actuator.Delete(ctx, pubip.DeepCopyObject().(client.Object))
			Expect(err).To(HaveOccurred())
			requeueAfterError, ok := err.(*controllererror.RequeueAfterError)
			Expect(ok).To(BeTrue())
			Expect(requeueAfterError.Cause).To(MatchError("public IP address still exists"))
			Expect(requeueAfterError.RequeueAfter).To(Equal(cfg.RequeueInterval.Duration))
		})

		It("should fail and requeue if getting the Azure IP address fails", func() {
			pubip := newPubip(true, nil, &earlyDeletionTimestamp, nil)
			failedOps := newFailedOps(azurev1alpha1.OperationTypeGetPublicIPAddress, 1, "could not get Azure public IP address by name: test")
			pubipWithFailedOps := newPubip(false, failedOps, &earlyDeletionTimestamp, nil)
			pubipUtils.EXPECT().GetByName(ctx, azurePublicIPAddressName).Return(nil, errors.New("test"))
			c.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: pubipName}, pubip).Return(nil)
			sw.EXPECT().Update(ctx, pubipWithFailedOps).Return(nil)

			_, err := actuator.Delete(ctx, pubip.DeepCopyObject().(client.Object))
			Expect(err).To(HaveOccurred())
			requeueAfterError, ok := err.(*controllererror.RequeueAfterError)
			Expect(ok).To(BeTrue())
			Expect(requeueAfterError.Cause).To(MatchError("could not get Azure public IP address by name: test"))
			Expect(requeueAfterError.RequeueAfter).To(Equal(cfg.RequeueInterval.Duration))
		})

		It("should not fail if getting the Azure IP address fails and max attempts have been reached", func() {
			failedOps := newFailedOps(azurev1alpha1.OperationTypeGetPublicIPAddress, cfg.MaxGetAttempts-1, "could not get Azure public IP address by name: test")
			pubip := newPubip(true, failedOps, &earlyDeletionTimestamp, nil)
			failedOps2 := newFailedOps(azurev1alpha1.OperationTypeGetPublicIPAddress, cfg.MaxGetAttempts, "could not get Azure public IP address by name: test")
			pubip2 := newPubip(false, failedOps2, &earlyDeletionTimestamp, nil)
			pubipUtils.EXPECT().GetByName(ctx, azurePublicIPAddressName).Return(nil, errors.New("test"))
			c.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: pubipName}, pubip).Return(nil)
			sw.EXPECT().Update(ctx, pubip2).Return(nil)

			requeueAfter, err := actuator.Delete(ctx, pubip.DeepCopyObject().(client.Object))
			Expect(err).NotTo(HaveOccurred())
			Expect(requeueAfter).To(Equal(syncPeriod))
		})

		It("should fail and requeue if removing the Azure IP from the load balancer fails", func() {
			pubip := newPubip(true, nil, &earlyDeletionTimestamp, nil)
			failedOps := newFailedOps(azurev1alpha1.OperationTypeCleanPublicIPAddress, 1, "could not remove Azure public IP address from the load balancer: test")
			pubipWithFailedOps := newPubip(true, failedOps, &earlyDeletionTimestamp, nil)
			azurePublicIPAddress := newAzurePublicIPAddress(ip, true)
			pubipUtils.EXPECT().GetByName(ctx, azurePublicIPAddressName).Return(azurePublicIPAddress, nil)
			c.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: pubipName}, pubip).Return(nil)
			pubipUtils.EXPECT().RemoveFromLoadBalancer(ctx, []string{string(azurePublicIPAddressID)}).Return(errors.New("test"))
			c.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: pubipName}, pubip).Return(nil)
			sw.EXPECT().Update(ctx, pubipWithFailedOps).Return(nil)

			_, err := actuator.Delete(ctx, pubip.DeepCopyObject().(client.Object))
			Expect(err).To(HaveOccurred())
			requeueAfterError, ok := err.(*controllererror.RequeueAfterError)
			Expect(ok).To(BeTrue())
			Expect(requeueAfterError.Cause).To(MatchError("could not remove Azure public IP address from the load balancer: test"))
			Expect(requeueAfterError.RequeueAfter).To(Equal(cfg.RequeueInterval.Duration))
		})

		It("should fail and requeue if deleting the Azure IP fails", func() {
			pubip := newPubip(true, nil, &earlyDeletionTimestamp, nil)
			failedOps := newFailedOps(azurev1alpha1.OperationTypeCleanPublicIPAddress, 1, "could not delete Azure public IP address: test")
			pubipWithFailedOps := newPubip(true, failedOps, &earlyDeletionTimestamp, nil)
			azurePublicIPAddress := newAzurePublicIPAddress(ip, true)
			pubipUtils.EXPECT().GetByName(ctx, azurePublicIPAddressName).Return(azurePublicIPAddress, nil)
			c.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: pubipName}, pubip).Return(nil)
			pubipUtils.EXPECT().RemoveFromLoadBalancer(ctx, []string{string(azurePublicIPAddressID)}).Return(nil)
			pubipUtils.EXPECT().Delete(ctx, azurePublicIPAddressName).Return(errors.New("test"))
			c.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: pubipName}, pubip).Return(nil)
			sw.EXPECT().Update(ctx, pubipWithFailedOps).Return(nil)

			_, err := actuator.Delete(ctx, pubip.DeepCopyObject().(client.Object))
			Expect(err).To(HaveOccurred())
			requeueAfterError, ok := err.(*controllererror.RequeueAfterError)
			Expect(ok).To(BeTrue())
			Expect(requeueAfterError.Cause).To(MatchError("could not delete Azure public IP address: test"))
			Expect(requeueAfterError.RequeueAfter).To(Equal(cfg.RequeueInterval.Duration))
		})

		It("should not fail if deleting the Azure IP address fails and max attempts have been reached", func() {
			failedOps := newFailedOps(azurev1alpha1.OperationTypeCleanPublicIPAddress, cfg.MaxCleanAttempts-1, "could not delete Azure public IP address: test")
			pubip := newPubip(true, failedOps, &earlyDeletionTimestamp, nil)
			failedOps2 := newFailedOps(azurev1alpha1.OperationTypeCleanPublicIPAddress, cfg.MaxCleanAttempts, "could not delete Azure public IP address: test")
			pubip2 := newPubip(true, failedOps2, &earlyDeletionTimestamp, nil)
			azurePublicIPAddress := newAzurePublicIPAddress(ip, true)
			pubipUtils.EXPECT().GetByName(ctx, azurePublicIPAddressName).Return(azurePublicIPAddress, nil)
			c.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: pubipName}, pubip).Return(nil)
			pubipUtils.EXPECT().RemoveFromLoadBalancer(ctx, []string{string(azurePublicIPAddressID)}).Return(nil)
			pubipUtils.EXPECT().Delete(ctx, azurePublicIPAddressName).Return(errors.New("test"))
			c.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: pubipName}, pubip).Return(nil)
			sw.EXPECT().Update(ctx, pubip2).Return(nil)

			requeueAfter, err := actuator.Delete(ctx, pubip.DeepCopyObject().(client.Object))
			Expect(err).NotTo(HaveOccurred())
			Expect(requeueAfter).To(Equal(syncPeriod))
		})
	})
})
