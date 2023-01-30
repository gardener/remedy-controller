// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

	"github.com/gardener/remedy-controller/pkg/controller"
	mockclient "github.com/gardener/remedy-controller/pkg/mock/controller-runtime/client"
	mockcontroller "github.com/gardener/remedy-controller/pkg/mock/remedy-controller/controller"

	"github.com/go-logr/logr"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

const (
	finalizer = "finalizer"
)

var _ = Describe("OwnedObjectPredicate", func() {
	var (
		ctrl *gomock.Controller

		m *mockcontroller.MockMapper
		r *mockclient.MockReader

		logger logr.Logger
		p      predicate.Predicate

		obj   *corev1.Pod
		owner *corev1.Service
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())

		m = mockcontroller.NewMockMapper(ctrl)
		r = mockclient.NewMockReader(ctrl)

		logger = log.Log.WithName("test")
		p = controller.NewOwnedObjectPredicate(&corev1.Service{}, r, m, finalizer, logger)

		obj = &corev1.Pod{}
		owner = &corev1.Service{}
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	var (
		ownerKey      = client.ObjectKey{Namespace: namespace, Name: name}
		now           = metav1.Now()
		notFoundError = &apierrors.StatusError{
			ErrStatus: metav1.Status{
				Reason: metav1.StatusReasonNotFound,
			},
		}

		expectGetOwner = func() {
			r.EXPECT().Get(context.Background(), ownerKey, gomock.AssignableToTypeOf(&corev1.Service{})).DoAndReturn(
				func(_ context.Context, _ client.ObjectKey, svc *corev1.Service) error {
					*svc = *owner
					return nil
				},
			)
		}
	)

	Describe("#Create", func() {
		It("should return false with an empty event", func() {
			Expect(p.Create(event.CreateEvent{})).To(Equal(false))
		})

		It("should return false if the mapper returns an empty owner key", func() {
			m.EXPECT().Map(obj).Return(client.ObjectKey{})

			Expect(p.Create(event.CreateEvent{Object: obj})).To(Equal(false))
		})

		It("should return false if getting the owner fails", func() {
			m.EXPECT().Map(obj).Return(ownerKey)
			r.EXPECT().Get(context.Background(), ownerKey, gomock.AssignableToTypeOf(&corev1.Service{})).Return(errors.New("test"))

			Expect(p.Create(event.CreateEvent{Object: obj})).To(Equal(false))
		})

		It("should return true if the owner is not found", func() {
			m.EXPECT().Map(obj).Return(ownerKey)
			r.EXPECT().Get(context.Background(), ownerKey, gomock.AssignableToTypeOf(&corev1.Service{})).Return(notFoundError)

			Expect(p.Create(event.CreateEvent{Object: obj})).To(Equal(true))
		})

		It("should return false if the owner doesn't have a finalizer", func() {
			m.EXPECT().Map(obj).Return(ownerKey)
			expectGetOwner()

			Expect(p.Create(event.CreateEvent{Object: obj})).To(Equal(false))
		})

		It("should return true if the owner has a finalizer and a deletion timestamp", func() {
			owner = &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					DeletionTimestamp: &now,
					Finalizers:        []string{finalizer},
				},
			}
			m.EXPECT().Map(obj).Return(ownerKey)
			expectGetOwner()

			Expect(p.Create(event.CreateEvent{Object: obj})).To(Equal(true))
		})

		It("should return false if the owner has a finalizer and no deletion timestamp", func() {
			owner = &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Finalizers: []string{finalizer},
				},
			}
			m.EXPECT().Map(obj).Return(ownerKey)
			expectGetOwner()

			Expect(p.Create(event.CreateEvent{Object: obj})).To(Equal(false))
		})
	})

	Describe("#Update", func() {
		It("should return false with an empty event", func() {
			Expect(p.Update(event.UpdateEvent{})).To(Equal(false))
		})

		It("should return false if the mapper returns an empty owner key", func() {
			m.EXPECT().Map(obj).Return(client.ObjectKey{})

			Expect(p.Update(event.UpdateEvent{ObjectNew: obj})).To(Equal(false))
		})

		It("should return false if getting the owner fails", func() {
			m.EXPECT().Map(obj).Return(ownerKey)
			r.EXPECT().Get(context.Background(), ownerKey, gomock.AssignableToTypeOf(&corev1.Service{})).Return(errors.New("test"))

			Expect(p.Update(event.UpdateEvent{ObjectNew: obj})).To(Equal(false))
		})

		It("should return true if the owner is not found", func() {
			m.EXPECT().Map(obj).Return(ownerKey)
			r.EXPECT().Get(context.Background(), ownerKey, gomock.AssignableToTypeOf(&corev1.Service{})).Return(notFoundError)

			Expect(p.Update(event.UpdateEvent{ObjectNew: obj})).To(Equal(true))
		})

		It("should return false if the owner doesn't have a finalizer", func() {
			m.EXPECT().Map(obj).Return(ownerKey)
			expectGetOwner()

			Expect(p.Update(event.UpdateEvent{ObjectNew: obj})).To(Equal(false))
		})

		It("should return true if the owner has a finalizer and a deletion timestamp", func() {
			owner = &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					DeletionTimestamp: &now,
					Finalizers:        []string{finalizer},
				},
			}
			m.EXPECT().Map(obj).Return(ownerKey)
			expectGetOwner()

			Expect(p.Update(event.UpdateEvent{ObjectNew: obj})).To(Equal(true))
		})

		It("should return false if the owner has a finalizer and no deletion timestamp", func() {
			owner = &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Finalizers: []string{finalizer},
				},
			}
			m.EXPECT().Map(obj).Return(ownerKey)
			expectGetOwner()

			Expect(p.Update(event.UpdateEvent{ObjectNew: obj})).To(Equal(false))
		})
	})

	Describe("#Delete", func() {
		It("should return false with an empty event", func() {
			Expect(p.Delete(event.DeleteEvent{})).To(Equal(false))
		})

		It("should return false if the mapper returns an empty owner key", func() {
			m.EXPECT().Map(obj).Return(client.ObjectKey{})

			Expect(p.Delete(event.DeleteEvent{Object: obj})).To(Equal(false))
		})

		It("should return false if getting the owner fails", func() {
			m.EXPECT().Map(obj).Return(ownerKey)
			r.EXPECT().Get(context.Background(), ownerKey, gomock.AssignableToTypeOf(&corev1.Service{})).Return(errors.New("test"))

			Expect(p.Delete(event.DeleteEvent{Object: obj})).To(Equal(false))
		})

		It("should return false if the owner is not found", func() {
			m.EXPECT().Map(obj).Return(ownerKey)
			r.EXPECT().Get(context.Background(), ownerKey, gomock.AssignableToTypeOf(&corev1.Service{})).Return(notFoundError)

			Expect(p.Delete(event.DeleteEvent{Object: obj})).To(Equal(false))
		})

		It("should return false if the owner doesn't have a finalizer", func() {
			m.EXPECT().Map(obj).Return(ownerKey)
			expectGetOwner()

			Expect(p.Delete(event.DeleteEvent{Object: obj})).To(Equal(false))
		})

		It("should return true if the owner has a finalizer and no deletion timestamp", func() {
			owner = &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Finalizers: []string{finalizer},
				},
			}
			m.EXPECT().Map(obj).Return(ownerKey)
			expectGetOwner()

			Expect(p.Delete(event.DeleteEvent{Object: obj})).To(Equal(true))
		})

		It("should return false if the owner has a finalizer and a deletion timestamp", func() {
			owner = &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					DeletionTimestamp: &now,
					Finalizers:        []string{finalizer},
				},
			}
			m.EXPECT().Map(obj).Return(ownerKey)
			expectGetOwner()

			Expect(p.Delete(event.DeleteEvent{Object: obj})).To(Equal(false))
		})
	})
})
