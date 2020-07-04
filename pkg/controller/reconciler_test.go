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

package controller_test

import (
	"context"
	"errors"
	"time"

	"github.com/go-logr/logr"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/runtime/inject"

	"github.com/gardener/remedy-controller/pkg/controller"
	mockclient "github.com/gardener/remedy-controller/pkg/mock/controller-runtime/client"
	mockmanager "github.com/gardener/remedy-controller/pkg/mock/controller-runtime/manager"
	mockcontroller "github.com/gardener/remedy-controller/pkg/mock/remedy-controller/controller"
)

var _ = Describe("Controller", func() {
	var (
		ctrl *gomock.Controller

		m *mockmanager.MockManager
		a *mockcontroller.MockActuator
		c *mockclient.MockClient

		logger     logr.Logger
		reconciler reconcile.Reconciler

		ts                                   metav1.Time
		request                              reconcile.Request
		obj                                  runtime.Object
		objWithDeletionTimestamp             runtime.Object
		objWithNoFinalizer                   runtime.Object
		objWithFinalizer                     runtime.Object
		objWithDeletionTimestampAndFinalizer runtime.Object
		notFoundError                        apierrors.APIStatus
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())

		m = mockmanager.NewMockManager(ctrl)
		a = mockcontroller.NewMockActuator(ctrl)
		c = mockclient.NewMockClient(ctrl)

		logger = log.Log.WithName("test")
		reconciler = controller.NewReconciler(m, a, "test-controller", "test-finalizer", &corev1.Pod{}, logger)
		Expect(reconciler.(inject.Client).InjectClient(c)).To(Succeed())

		ts = metav1.Now()
		request = reconcile.Request{
			NamespacedName: types.NamespacedName{
				Namespace: "test-namespace",
				Name:      "test-name",
			},
		}
		obj = &corev1.Pod{}
		objWithDeletionTimestamp = &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				DeletionTimestamp: &ts,
				Finalizers:        []string{},
			},
		}
		objWithNoFinalizer = &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Finalizers: []string{},
			},
		}
		objWithFinalizer = &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Finalizers: []string{"test-finalizer"},
			},
		}
		objWithDeletionTimestampAndFinalizer = &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				DeletionTimestamp: &ts,
				Finalizers:        []string{"test-finalizer"},
			},
		}
		notFoundError = &apierrors.StatusError{
			ErrStatus: metav1.Status{
				Reason: metav1.StatusReasonNotFound,
			},
		}
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#Reconcile", func() {
		It("should create or update an object if it should be finalized", func() {
			c.EXPECT().Get(gomock.Any(), request.NamespacedName, obj).Return(nil)
			a.EXPECT().ShouldFinalize(gomock.Any(), obj).Return(true, nil)
			c.EXPECT().Update(gomock.Any(), objWithFinalizer).Return(nil)
			a.EXPECT().CreateOrUpdate(gomock.Any(), objWithFinalizer).Return(time.Duration(0), nil)

			result, err := reconciler.Reconcile(request)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(reconcile.Result{}))
		})

		It("should not create or update an object if it should not be finalized and doesn't have a finalizer", func() {
			c.EXPECT().Get(gomock.Any(), request.NamespacedName, obj).Return(nil)
			a.EXPECT().ShouldFinalize(gomock.Any(), obj).Return(false, nil)

			result, err := reconciler.Reconcile(request)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(reconcile.Result{}))
		})

		It("should create or update an object and remove the finalizer if it should not be finalized but already has a finalizer", func() {
			c.EXPECT().Get(gomock.Any(), request.NamespacedName, obj).DoAndReturn(func(_ context.Context, _ client.ObjectKey, pod *corev1.Pod) error {
				pod.ObjectMeta.Finalizers = []string{"test-finalizer"}
				return nil
			})
			a.EXPECT().ShouldFinalize(gomock.Any(), objWithFinalizer).Return(false, nil)
			a.EXPECT().CreateOrUpdate(gomock.Any(), objWithFinalizer).Return(time.Duration(0), nil)
			c.EXPECT().Update(gomock.Any(), objWithNoFinalizer).Return(nil)

			result, err := reconciler.Reconcile(request)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(reconcile.Result{}))
		})

		It("should delete an object that has a finalizer", func() {
			c.EXPECT().Get(gomock.Any(), request.NamespacedName, obj).DoAndReturn(func(_ context.Context, _ client.ObjectKey, pod *corev1.Pod) error {
				pod.ObjectMeta.DeletionTimestamp = &ts
				pod.ObjectMeta.Finalizers = []string{"test-finalizer"}
				return nil
			})
			a.EXPECT().Delete(gomock.Any(), objWithDeletionTimestampAndFinalizer).Return(nil)
			c.EXPECT().Update(gomock.Any(), objWithDeletionTimestamp).Return(nil)

			result, err := reconciler.Reconcile(request)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(reconcile.Result{}))
		})

		It("should not delete an object that doesn't have a finalizer", func() {
			c.EXPECT().Get(gomock.Any(), request.NamespacedName, obj).DoAndReturn(func(_ context.Context, _ client.ObjectKey, pod *corev1.Pod) error {
				pod.ObjectMeta.DeletionTimestamp = &ts
				return nil
			})

			result, err := reconciler.Reconcile(request)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(reconcile.Result{}))
		})

		It("should do nothing if the object does not exist", func() {
			c.EXPECT().Get(gomock.Any(), request.NamespacedName, obj).Return(notFoundError)

			result, err := reconciler.Reconcile(request)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(reconcile.Result{}))
		})

		It("should fail if the actuator fails to check if the object should be finalized", func() {
			c.EXPECT().Get(gomock.Any(), request.NamespacedName, obj).Return(nil)
			a.EXPECT().ShouldFinalize(gomock.Any(), obj).Return(false, errors.New("test"))

			_, err := reconciler.Reconcile(request)
			Expect(err).To(MatchError("could not check if the object should be finalized: test"))
		})

		It("should fail if the actuator fails to create or update", func() {
			c.EXPECT().Get(gomock.Any(), request.NamespacedName, obj).Return(nil)
			a.EXPECT().ShouldFinalize(gomock.Any(), obj).Return(true, nil)
			c.EXPECT().Update(gomock.Any(), objWithFinalizer).Return(nil)
			a.EXPECT().CreateOrUpdate(gomock.Any(), objWithFinalizer).Return(time.Duration(0), errors.New("test"))

			_, err := reconciler.Reconcile(request)
			Expect(err).To(MatchError("could not reconcile object creation or update: test"))
		})

		It("should fail if the actuator fails to delete", func() {
			c.EXPECT().Get(gomock.Any(), request.NamespacedName, obj).DoAndReturn(func(_ context.Context, _ client.ObjectKey, pod *corev1.Pod) error {
				pod.ObjectMeta.DeletionTimestamp = &ts
				pod.ObjectMeta.Finalizers = []string{"test-finalizer"}
				return nil
			})
			a.EXPECT().Delete(gomock.Any(), objWithDeletionTimestampAndFinalizer).Return(errors.New("test"))

			_, err := reconciler.Reconcile(request)
			Expect(err).To(MatchError("could not reconcile object deletion: test"))
		})
	})
})
