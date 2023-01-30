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

package node_test

import (
	"context"
	"errors"
	"time"

	azurev1alpha1 "github.com/gardener/remedy-controller/pkg/apis/azure/v1alpha1"
	"github.com/gardener/remedy-controller/pkg/controller"
	"github.com/gardener/remedy-controller/pkg/controller/azure"
	azurenode "github.com/gardener/remedy-controller/pkg/controller/azure/node"
	mockclient "github.com/gardener/remedy-controller/pkg/mock/controller-runtime/client"

	"github.com/go-logr/logr"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
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
		nodeName   = "test-node"
		hostname   = "test-hostname"
		providerID = "test-provider-id"
		namespace  = "default"

		syncPeriod = 1 * time.Minute
	)

	var (
		ctrl *gomock.Controller
		ctx  context.Context

		c *mockclient.MockClient

		logger   logr.Logger
		actuator controller.Actuator

		node     *corev1.Node
		vmLabels map[string]string
		emptyVM  *azurev1alpha1.VirtualMachine
		vm       *azurev1alpha1.VirtualMachine
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		ctx = context.TODO()

		c = mockclient.NewMockClient(ctrl)

		logger = log.Log.WithName("test")
		actuator = azurenode.NewActuator(c, namespace, syncPeriod, logger)

		node = &corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name: nodeName,
				Labels: map[string]string{
					azurenode.HostnameLabel: hostname,
				},
			},
			Spec: corev1.NodeSpec{
				ProviderID: providerID,
			},
			Status: corev1.NodeStatus{
				Conditions: []corev1.NodeCondition{
					{
						Type:   corev1.NodeReady,
						Status: corev1.ConditionTrue,
					},
				},
			},
		}
		vmLabels = map[string]string{
			azure.NodeLabel: nodeName,
		}
		emptyVM = &azurev1alpha1.VirtualMachine{
			ObjectMeta: metav1.ObjectMeta{
				Name:      nodeName,
				Namespace: namespace,
			},
		}
		vm = &azurev1alpha1.VirtualMachine{
			ObjectMeta: metav1.ObjectMeta{
				Name:      nodeName,
				Namespace: namespace,
				Labels:    vmLabels,
			},
			Spec: azurev1alpha1.VirtualMachineSpec{
				Hostname:              hostname,
				ProviderID:            providerID,
				NotReadyOrUnreachable: false,
			},
		}
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#CreateOrUpdate", func() {
		It("should create the VirtualMachine object for a node if it doesn't exist", func() {
			c.EXPECT().Get(ctx, client.ObjectKey{Namespace: vm.Namespace, Name: vm.Name}, emptyVM).
				Return(apierrors.NewNotFound(schema.GroupResource{}, vm.Name))
			c.EXPECT().Create(ctx, vm).Return(nil)

			requeueAfter, err := actuator.CreateOrUpdate(ctx, node)
			Expect(err).NotTo(HaveOccurred())
			Expect(requeueAfter).To(Equal(syncPeriod))
		})

		It("should fail when creating the VirtualMachine object for a node and an error occurs", func() {
			c.EXPECT().Get(ctx, client.ObjectKey{Namespace: vm.Namespace, Name: vm.Name}, emptyVM).
				Return(apierrors.NewNotFound(schema.GroupResource{}, vm.Name))
			c.EXPECT().Create(ctx, vm).Return(apierrors.NewInternalError(errors.New("test")))

			_, err := actuator.CreateOrUpdate(ctx, node)
			Expect(err).To(MatchError("could not create or update virtualmachine: Internal error occurred: test"))
		})

		It("should update the VirtualMachine object for a node if it already exists and is not properly initialized", func() {
			c.EXPECT().Get(ctx, client.ObjectKey{Namespace: vm.Namespace, Name: vm.Name}, emptyVM).
				DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *azurev1alpha1.VirtualMachine) error {
					obj.Spec.Hostname = "unknown"
					return nil
				})
			c.EXPECT().Update(ctx, vm).Return(nil)

			requeueAfter, err := actuator.CreateOrUpdate(ctx, node)
			Expect(err).NotTo(HaveOccurred())
			Expect(requeueAfter).To(Equal(syncPeriod))
		})

		It("should not update the VirtualMachine object for a node if it already exists and is properly initialized", func() {
			c.EXPECT().Get(ctx, client.ObjectKey{Namespace: vm.Namespace, Name: vm.Name}, emptyVM).
				DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *azurev1alpha1.VirtualMachine) error {
					*obj = *vm
					return nil
				})

			requeueAfter, err := actuator.CreateOrUpdate(ctx, node)
			Expect(err).NotTo(HaveOccurred())
			Expect(requeueAfter).To(Equal(syncPeriod))
		})

		It("should retry when updating the VirtualMachine object for a node and a Conflict error occurs", func() {
			c.EXPECT().Get(ctx, client.ObjectKey{Namespace: vm.Namespace, Name: vm.Name}, emptyVM).
				DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *azurev1alpha1.VirtualMachine) error {
					obj.Spec.Hostname = "unknown"
					return nil
				})
			c.EXPECT().Update(ctx, vm).Return(apierrors.NewConflict(schema.GroupResource{}, vm.Name, nil))
			c.EXPECT().Get(ctx, client.ObjectKey{Namespace: vm.Namespace, Name: vm.Name}, vm).
				DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *azurev1alpha1.VirtualMachine) error {
					obj.Spec.Hostname = "unknown"
					return nil
				})
			c.EXPECT().Update(ctx, vm).Return(nil)

			requeueAfter, err := actuator.CreateOrUpdate(ctx, node)
			Expect(err).NotTo(HaveOccurred())
			Expect(requeueAfter).To(Equal(syncPeriod))
		})

		It("should fail when updating the VirtualMachine object for a node and an error different from Conflict occurs", func() {
			c.EXPECT().Get(ctx, client.ObjectKey{Namespace: vm.Namespace, Name: vm.Name}, emptyVM).
				DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *azurev1alpha1.VirtualMachine) error {
					obj.Spec.Hostname = "unknown"
					return nil
				})
			c.EXPECT().Update(ctx, vm).Return(apierrors.NewInternalError(errors.New("test")))

			_, err := actuator.CreateOrUpdate(ctx, node)
			Expect(err).To(MatchError("could not create or update virtualmachine: Internal error occurred: test"))
		})
	})

	Describe("#Delete", func() {
		It("should delete the VirtualMachine object for a node", func() {
			c.EXPECT().Delete(ctx, emptyVM).Return(nil)

			requeueAfter, err := actuator.Delete(ctx, node)
			Expect(err).NotTo(HaveOccurred())
			Expect(requeueAfter).To(Equal(time.Duration(0)))
		})

		It("should succeed when deleting the VirtualMachine object for a node and a NotFound error occurs", func() {
			c.EXPECT().Delete(ctx, emptyVM).Return(apierrors.NewNotFound(schema.GroupResource{}, vm.Name))

			requeueAfter, err := actuator.Delete(ctx, node)
			Expect(err).NotTo(HaveOccurred())
			Expect(requeueAfter).To(Equal(time.Duration(0)))
		})

		It("should fail when deleting the VirtualMachine object for a node and an error different from NotFound occurs", func() {
			c.EXPECT().Delete(ctx, emptyVM).Return(apierrors.NewInternalError(errors.New("test")))

			_, err := actuator.Delete(ctx, node)
			Expect(err).To(MatchError("could not delete virtualmachine: Internal error occurred: test"))
		})
	})
})
