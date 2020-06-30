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

package service_test

import (
	"context"
	"errors"
	"time"

	azurev1alpha1 "github.com/gardener/remedy-controller/pkg/apis/azure/v1alpha1"
	"github.com/gardener/remedy-controller/pkg/controller"
	azureservice "github.com/gardener/remedy-controller/pkg/controller/azure/service"
	mockclient "github.com/gardener/remedy-controller/pkg/mock/controller-runtime/client"

	"github.com/go-logr/logr"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

var _ = Describe("Actuator", func() {
	const (
		serviceName      = "test-service"
		serviceNamespace = "test"
		namespace        = "default"
		ip               = "1.2.3.4"
	)

	var (
		ctrl *gomock.Controller
		ctx  context.Context

		c *mockclient.MockClient

		logger   logr.Logger
		actuator controller.Actuator

		svc          *corev1.Service
		clusterIPSvc *corev1.Service
		pubipLabels  map[string]string
		emptyPubip   *azurev1alpha1.PublicIPAddress
		pubip        *azurev1alpha1.PublicIPAddress
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		ctx = context.TODO()

		c = mockclient.NewMockClient(ctrl)

		logger = log.Log.WithName("test")
		actuator = azureservice.NewActuator(c, namespace, logger)

		svc = &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      serviceName,
				Namespace: serviceNamespace,
			},
			Spec: corev1.ServiceSpec{
				Type: corev1.ServiceTypeLoadBalancer,
			},
			Status: corev1.ServiceStatus{
				LoadBalancer: corev1.LoadBalancerStatus{
					Ingress: []corev1.LoadBalancerIngress{
						{IP: ip},
					},
				},
			},
		}
		clusterIPSvc = &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      serviceName,
				Namespace: serviceNamespace,
			},
			Spec: corev1.ServiceSpec{
				Type: corev1.ServiceTypeClusterIP,
			},
		}
		pubipLabels = map[string]string{
			azureservice.Label: serviceNamespace + "." + serviceName,
		}
		emptyPubip = &azurev1alpha1.PublicIPAddress{
			ObjectMeta: metav1.ObjectMeta{
				Name:      serviceNamespace + "-" + serviceName + "-" + ip,
				Namespace: namespace,
			},
		}
		pubip = &azurev1alpha1.PublicIPAddress{
			ObjectMeta: metav1.ObjectMeta{
				Name:      serviceNamespace + "-" + serviceName + "-" + ip,
				Namespace: namespace,
				Labels:    pubipLabels,
			},
			Spec: azurev1alpha1.PublicIPAddressSpec{
				IPAddress: ip,
			},
		}
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#CreateOrUpdate", func() {
		It("should create the PublicIPAddress object for an existing service IP if it doesn't exist", func() {
			c.EXPECT().Get(ctx, client.ObjectKey{Namespace: pubip.Namespace, Name: pubip.Name}, emptyPubip).
				Return(apierrors.NewNotFound(schema.GroupResource{}, pubip.Name))
			c.EXPECT().Create(ctx, pubip).Return(nil)
			c.EXPECT().List(ctx, &azurev1alpha1.PublicIPAddressList{}, client.InNamespace(namespace), client.MatchingLabels(pubipLabels)).
				DoAndReturn(func(_ context.Context, list *azurev1alpha1.PublicIPAddressList, _ ...client.ListOption) error {
					list.Items = append(list.Items, *pubip)
					return nil
				})

			requeueAfter, removeFinalizer, err := actuator.CreateOrUpdate(ctx, svc)
			Expect(err).NotTo(HaveOccurred())
			Expect(requeueAfter).To(Equal(time.Duration(0)))
			Expect(removeFinalizer).To(Equal(false))
		})

		It("should fail when creating the PublicIPAddress object for an existing service IP and an error occurs", func() {
			c.EXPECT().Get(ctx, client.ObjectKey{Namespace: pubip.Namespace, Name: pubip.Name}, emptyPubip).
				Return(apierrors.NewNotFound(schema.GroupResource{}, pubip.Name))
			c.EXPECT().Create(ctx, pubip).Return(apierrors.NewInternalError(errors.New("test")))

			_, _, err := actuator.CreateOrUpdate(ctx, svc)
			Expect(err).To(MatchError("could not create or update publicipaddress: Internal error occurred: test"))
		})

		It("should fail when an error occurs while listing PublicIPAddress objects", func() {
			c.EXPECT().Get(ctx, client.ObjectKey{Namespace: pubip.Namespace, Name: pubip.Name}, emptyPubip).
				Return(apierrors.NewNotFound(schema.GroupResource{}, pubip.Name))
			c.EXPECT().Create(ctx, pubip).Return(nil)
			c.EXPECT().List(ctx, &azurev1alpha1.PublicIPAddressList{}, client.InNamespace(namespace), client.MatchingLabels(pubipLabels)).
				Return(apierrors.NewInternalError(errors.New("test")))

			_, _, err := actuator.CreateOrUpdate(ctx, svc)
			Expect(err).To(MatchError("could not list publicipaddresses: Internal error occurred: test"))
		})

		It("should update the PublicIPAddress object for an existing service IP if it already exists and is not properly initialized", func() {
			c.EXPECT().Get(ctx, client.ObjectKey{Namespace: pubip.Namespace, Name: pubip.Name}, emptyPubip).
				DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *azurev1alpha1.PublicIPAddress) error {
					obj.Spec.IPAddress = "0.0.0.0"
					return nil
				})
			c.EXPECT().Update(ctx, pubip).Return(nil)
			c.EXPECT().List(ctx, &azurev1alpha1.PublicIPAddressList{}, client.InNamespace(namespace), client.MatchingLabels(pubipLabels)).
				DoAndReturn(func(_ context.Context, list *azurev1alpha1.PublicIPAddressList, _ ...client.ListOption) error {
					list.Items = append(list.Items, *pubip)
					return nil
				})

			requeueAfter, removeFinalizer, err := actuator.CreateOrUpdate(ctx, svc)
			Expect(err).NotTo(HaveOccurred())
			Expect(requeueAfter).To(Equal(time.Duration(0)))
			Expect(removeFinalizer).To(Equal(false))
		})

		It("should not update the PublicIPAddress object for an existing service IP if it already exists and is properly initialized", func() {
			c.EXPECT().Get(ctx, client.ObjectKey{Namespace: pubip.Namespace, Name: pubip.Name}, emptyPubip).
				DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *azurev1alpha1.PublicIPAddress) error {
					*obj = *pubip
					return nil
				})
			c.EXPECT().List(ctx, &azurev1alpha1.PublicIPAddressList{}, client.InNamespace(namespace), client.MatchingLabels(pubipLabels)).
				DoAndReturn(func(_ context.Context, list *azurev1alpha1.PublicIPAddressList, _ ...client.ListOption) error {
					list.Items = append(list.Items, *pubip)
					return nil
				})

			requeueAfter, removeFinalizer, err := actuator.CreateOrUpdate(ctx, svc)
			Expect(err).NotTo(HaveOccurred())
			Expect(requeueAfter).To(Equal(time.Duration(0)))
			Expect(removeFinalizer).To(Equal(false))
		})

		It("should retry when updating the PublicIPAddress object for an existing service IP and a Conflict error occurs", func() {
			c.EXPECT().Get(ctx, client.ObjectKey{Namespace: pubip.Namespace, Name: pubip.Name}, emptyPubip).
				DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *azurev1alpha1.PublicIPAddress) error {
					obj.Spec.IPAddress = "0.0.0.0"
					return nil
				})
			c.EXPECT().Update(ctx, pubip).Return(apierrors.NewConflict(schema.GroupResource{}, pubip.Name, nil))
			c.EXPECT().Get(ctx, client.ObjectKey{Namespace: pubip.Namespace, Name: pubip.Name}, pubip).
				DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *azurev1alpha1.PublicIPAddress) error {
					obj.Spec.IPAddress = "1.1.1.1"
					return nil
				})
			c.EXPECT().Update(ctx, pubip).Return(nil)
			c.EXPECT().List(ctx, &azurev1alpha1.PublicIPAddressList{}, client.InNamespace(namespace), client.MatchingLabels(pubipLabels)).
				DoAndReturn(func(_ context.Context, list *azurev1alpha1.PublicIPAddressList, _ ...client.ListOption) error {
					list.Items = append(list.Items, *pubip)
					return nil
				})

			requeueAfter, removeFinalizer, err := actuator.CreateOrUpdate(ctx, svc)
			Expect(err).NotTo(HaveOccurred())
			Expect(requeueAfter).To(Equal(time.Duration(0)))
			Expect(removeFinalizer).To(Equal(false))
		})

		It("should fail when updating the PublicIPAddress object for an existing service IP and an error different from Conflict occurs", func() {
			c.EXPECT().Get(ctx, client.ObjectKey{Namespace: pubip.Namespace, Name: pubip.Name}, emptyPubip).
				DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *azurev1alpha1.PublicIPAddress) error {
					obj.Spec.IPAddress = "0.0.0.0"
					return nil
				})
			c.EXPECT().Update(ctx, pubip).Return(apierrors.NewInternalError(errors.New("test")))

			_, _, err := actuator.CreateOrUpdate(ctx, svc)
			Expect(err).To(MatchError("could not create or update publicipaddress: Internal error occurred: test"))
		})

		It("should delete the PublicIPAddress object for a non-existing service IP", func() {
			c.EXPECT().List(ctx, &azurev1alpha1.PublicIPAddressList{}, client.InNamespace(namespace), client.MatchingLabels(pubipLabels)).
				DoAndReturn(func(_ context.Context, list *azurev1alpha1.PublicIPAddressList, _ ...client.ListOption) error {
					list.Items = append(list.Items, *pubip)
					return nil
				})
			c.EXPECT().Delete(ctx, pubip).Return(nil)

			requeueAfter, removeFinalizer, err := actuator.CreateOrUpdate(ctx, clusterIPSvc)
			Expect(err).NotTo(HaveOccurred())
			Expect(requeueAfter).To(Equal(time.Duration(0)))
			Expect(removeFinalizer).To(Equal(true))
		})

		It("should succeed when deleting the PublicIPAddress object for a non-existing service IP and a NotFound error occurs", func() {
			c.EXPECT().List(ctx, &azurev1alpha1.PublicIPAddressList{}, client.InNamespace(namespace), client.MatchingLabels(pubipLabels)).
				DoAndReturn(func(_ context.Context, list *azurev1alpha1.PublicIPAddressList, _ ...client.ListOption) error {
					list.Items = append(list.Items, *pubip)
					return nil
				})
			c.EXPECT().Delete(ctx, pubip).Return(apierrors.NewNotFound(schema.GroupResource{}, pubip.Name))

			requeueAfter, removeFinalizer, err := actuator.CreateOrUpdate(ctx, clusterIPSvc)
			Expect(err).NotTo(HaveOccurred())
			Expect(requeueAfter).To(Equal(time.Duration(0)))
			Expect(removeFinalizer).To(Equal(true))
		})

		It("should fail when deleting the PublicIPAddress object for a non-existing service IP an error different from NotFound occurs", func() {
			c.EXPECT().List(ctx, &azurev1alpha1.PublicIPAddressList{}, client.InNamespace(namespace), client.MatchingLabels(pubipLabels)).
				DoAndReturn(func(_ context.Context, list *azurev1alpha1.PublicIPAddressList, _ ...client.ListOption) error {
					list.Items = append(list.Items, *pubip)
					return nil
				})
			c.EXPECT().Delete(ctx, pubip).Return(apierrors.NewInternalError(errors.New("test")))

			_, _, err := actuator.CreateOrUpdate(ctx, clusterIPSvc)
			Expect(err).To(MatchError("could not delete publicipaddress: Internal error occurred: test"))
		})
	})

	Describe("#Delete", func() {
		It("should delete the PublicIPAddress object for an existing service IP", func() {
			c.EXPECT().Delete(ctx, emptyPubip).Return(nil)

			Expect(actuator.Delete(ctx, svc)).To(Succeed())
		})

		It("should succeed when deleting the PublicIPAddress object for an existing service IP and a NotFound error occurs", func() {
			c.EXPECT().Delete(ctx, emptyPubip).Return(apierrors.NewNotFound(schema.GroupResource{}, pubip.Name))

			Expect(actuator.Delete(ctx, svc)).To(Succeed())
		})

		It("should fail when deleting the PublicIPAddress object for an existing service IP and an error different from NotFound occurs", func() {
			c.EXPECT().Delete(ctx, emptyPubip).Return(apierrors.NewInternalError(errors.New("test")))

			err := actuator.Delete(ctx, svc)
			Expect(err).To(MatchError("could not delete publicipaddress: Internal error occurred: test"))
		})
	})
})
