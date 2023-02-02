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
	"fmt"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/golang/mock/gomock"
	"github.com/google/go-cmp/cmp"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/runtime/inject"

	"github.com/gardener/remedy-controller/pkg/controller"
	mockclient "github.com/gardener/remedy-controller/pkg/mock/controller-runtime/client"
	mockcontroller "github.com/gardener/remedy-controller/pkg/mock/remedy-controller/controller"
)

const (
	name         = "test-name"
	namespace    = "test-namespace"
	requeueAfter = 1 * time.Minute
)

type eqMatcher struct {
	want interface{}
}

func eqMatch(want interface{}) eqMatcher {
	return eqMatcher{
		want: want,
	}
}

func (eq eqMatcher) Matches(got interface{}) bool {
	return gomock.Eq(eq.want).Matches(got)
}

func (eq eqMatcher) Got(got interface{}) string {
	return fmt.Sprintf("%v (%T)\nDiff (-got +want):\n%s", got, got, strings.TrimSpace(cmp.Diff(got, eq.want)))
}

func (eq eqMatcher) String() string {
	return fmt.Sprintf("%v (%T)\n", eq.want, eq.want)
}

var _ = Describe("Controller", func() {
	var (
		ctrl *gomock.Controller

		a *mockcontroller.MockActuator
		c *mockclient.MockClient

		ctx        context.Context
		logger     logr.Logger
		reconciler reconcile.Reconciler

		ts                                   metav1.Time
		request                              reconcile.Request
		obj                                  client.Object
		objWithDeletionTimestamp             client.Object
		objWithFinalizer                     client.Object
		objWithDeletionTimestampAndFinalizer client.Object
		notFoundError                        apierrors.APIStatus
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())

		a = mockcontroller.NewMockActuator(ctrl)
		c = mockclient.NewMockClient(ctrl)

		ctx = context.TODO()
		logger = log.Log.WithName("test")
		reconciler = controller.NewReconciler(a, "test-controller", "test-finalizer", &corev1.Pod{}, true, logger)
		Expect(reconciler.(inject.Client).InjectClient(c)).To(Succeed())
		Expect(reconciler.(inject.APIReader).InjectAPIReader(c)).To(Succeed())

		ts = metav1.Now()
		request = reconcile.Request{
			NamespacedName: types.NamespacedName{
				Namespace: namespace,
				Name:      name,
			},
		}
		obj = &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
			},
		}
		objWithDeletionTimestamp = &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:              name,
				Namespace:         namespace,
				DeletionTimestamp: &ts,
			},
		}
		objWithFinalizer = &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:       name,
				Namespace:  namespace,
				Finalizers: []string{"test-finalizer"},
			},
		}
		objWithDeletionTimestampAndFinalizer = &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:              name,
				Namespace:         namespace,
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
			c.EXPECT().Get(ctx, request.NamespacedName, obj).Return(nil)
			a.EXPECT().ShouldFinalize(ctx, obj).Return(true, nil)
			c.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, obj).Return(nil).AnyTimes()
			c.EXPECT().Patch(ctx, objWithFinalizer, gomock.Any()).Return(nil)
			a.EXPECT().CreateOrUpdate(ctx, objWithFinalizer).Return(requeueAfter, nil)

			result, err := reconciler.Reconcile(ctx, request)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(reconcile.Result{RequeueAfter: requeueAfter}))
		})

		It("should not create or update an object if it should not be finalized and doesn't have a finalizer", func() {
			c.EXPECT().Get(ctx, request.NamespacedName, obj).Return(nil)
			a.EXPECT().ShouldFinalize(ctx, obj).Return(false, nil)

			result, err := reconciler.Reconcile(ctx, request)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(reconcile.Result{}))
		})

		It("should create or update an object and remove the finalizer if it should not be finalized but already has a finalizer", func() {
			c.EXPECT().Get(gomock.Any(), request.NamespacedName, obj).DoAndReturn(func(_ context.Context, _ client.ObjectKey, pod *corev1.Pod, opts ...client.GetOption) error {
				pod.ObjectMeta.Finalizers = []string{"test-finalizer"}
				return nil
			})

			a.EXPECT().ShouldFinalize(ctx, objWithFinalizer).Return(false, nil)
			a.EXPECT().CreateOrUpdate(ctx, objWithFinalizer).Return(requeueAfter, nil)
			c.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, obj).Return(nil).AnyTimes()

			emptyFinalizerObj := obj.DeepCopyObject().(client.Object)
			emptyFinalizerObj.SetFinalizers([]string{})
			c.EXPECT().Patch(ctx, EqMatcher(emptyFinalizerObj), gomock.Any()).Return(nil)

			result, err := reconciler.Reconcile(ctx, request)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(reconcile.Result{}))
		})

		It("should delete an object that has a finalizer", func() {
			c.EXPECT().Get(ctx, request.NamespacedName, obj).DoAndReturn(func(_ context.Context, _ client.ObjectKey, pod *corev1.Pod, opts ...client.GetOption) error {
				pod.ObjectMeta.DeletionTimestamp = &ts
				pod.ObjectMeta.Finalizers = []string{"test-finalizer"}
				return nil
			})
			a.EXPECT().Delete(ctx, objWithDeletionTimestampAndFinalizer).Return(requeueAfter, nil)
			c.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, objWithDeletionTimestamp).Return(nil).AnyTimes()

			emptyFinalizerObj := objWithDeletionTimestamp.DeepCopyObject().(client.Object)
			emptyFinalizerObj.SetFinalizers([]string{})
			c.EXPECT().Patch(ctx, EqMatcher(emptyFinalizerObj), gomock.Any()).Return(nil)

			result, err := reconciler.Reconcile(ctx, request)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(reconcile.Result{RequeueAfter: requeueAfter}))
		})

		It("should not delete an object that doesn't have a finalizer", func() {
			c.EXPECT().Get(ctx, request.NamespacedName, obj).DoAndReturn(func(_ context.Context, _ client.ObjectKey, pod *corev1.Pod, opts ...client.GetOption) error {
				pod.ObjectMeta.DeletionTimestamp = &ts
				return nil
			})

			result, err := reconciler.Reconcile(ctx, request)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(reconcile.Result{}))
		})

		It("should ensure the deletion if the object does not exist", func() {
			c.EXPECT().Get(ctx, request.NamespacedName, obj).Return(notFoundError)
			a.EXPECT().Delete(ctx, obj).Return(requeueAfter, nil)

			result, err := reconciler.Reconcile(ctx, request)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(reconcile.Result{}))
		})

		It("should fail if the actuator fails to check if the object should be finalized", func() {
			c.EXPECT().Get(ctx, request.NamespacedName, obj).Return(nil)
			a.EXPECT().ShouldFinalize(ctx, obj).Return(false, errors.New("test"))

			_, err := reconciler.Reconcile(ctx, request)
			Expect(err).To(MatchError("could not check if the object should be finalized: test"))
		})

		It("should fail if the actuator fails to create or update", func() {
			c.EXPECT().Get(ctx, request.NamespacedName, obj).Return(nil)
			a.EXPECT().ShouldFinalize(ctx, obj).Return(true, nil)
			c.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, obj).Return(nil).AnyTimes()
			c.EXPECT().Patch(ctx, objWithFinalizer, gomock.Any()).Return(nil)
			a.EXPECT().CreateOrUpdate(ctx, objWithFinalizer).Return(time.Duration(0), errors.New("test"))

			_, err := reconciler.Reconcile(ctx, request)
			Expect(err).To(MatchError("could not reconcile object creation or update: test"))
		})

		It("should fail if the actuator fails to delete", func() {
			c.EXPECT().Get(ctx, request.NamespacedName, obj).DoAndReturn(func(_ context.Context, _ client.ObjectKey, pod *corev1.Pod, opts ...client.GetOption) error {
				pod.ObjectMeta.DeletionTimestamp = &ts
				pod.ObjectMeta.Finalizers = []string{"test-finalizer"}
				return nil
			})
			a.EXPECT().Delete(ctx, objWithDeletionTimestampAndFinalizer).Return(time.Duration(0), errors.New("test"))

			_, err := reconciler.Reconcile(ctx, request)
			Expect(err).To(MatchError("could not reconcile object deletion: test"))
		})

		It("should fail if the actuator fails to delete when ensuring the deletion", func() {
			c.EXPECT().Get(ctx, request.NamespacedName, obj).Return(notFoundError)
			a.EXPECT().Delete(ctx, obj).Return(time.Duration(0), errors.New("test"))

			_, err := reconciler.Reconcile(ctx, request)
			Expect(err).To(MatchError("could not ensure object deletion: test"))
		})
	})
})
