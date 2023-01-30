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

package service_test

import (
	"github.com/gardener/remedy-controller/pkg/controller/azure"
	azureservice "github.com/gardener/remedy-controller/pkg/controller/azure/service"
	mockutils "github.com/gardener/remedy-controller/pkg/mock/remedy-controller/utils"

	"github.com/go-logr/logr"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

var _ = Describe("Predicate", func() {
	const (
		serviceName = "test-service"
		ip          = "1.2.3.4"
	)

	var (
		ctrl *gomock.Controller

		serviceCache *mockutils.MockExpiringCache

		logger logr.Logger
		p      predicate.Predicate

		service    *corev1.Service
		projection *azureservice.Projection
		now        metav1.Time
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())

		serviceCache = mockutils.NewMockExpiringCache(ctrl)

		logger = log.Log.WithName("test")
		p = azureservice.NewPredicate(serviceCache, logger)

		service = &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name: serviceName,
			},
			Status: corev1.ServiceStatus{
				LoadBalancer: corev1.LoadBalancerStatus{
					Ingress: []corev1.LoadBalancerIngress{
						{
							IP: ip,
						},
					},
				},
			},
		}
		projection = &azureservice.Projection{
			LoadBalancerIPs: map[string]bool{
				ip: true,
			},
		}
		now = metav1.Now()
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#Create", func() {
		It("should return false with an empty event", func() {
			Expect(p.Create(event.CreateEvent{})).To(Equal(false))
		})

		It("should return false with an object that is not a service", func() {
			Expect(p.Create(event.CreateEvent{Object: &corev1.Node{}})).To(Equal(false))
		})

		It("should return true with an object that is a service (and add it to the service cache)", func() {
			serviceCache.EXPECT().Set(serviceName, projection, azureservice.CacheTTL)

			Expect(p.Create(event.CreateEvent{Object: service})).To(Equal(true))
		})
	})

	Describe("#Update", func() {
		It("should return false with an empty event", func() {
			Expect(p.Update(event.UpdateEvent{})).To(Equal(false))
		})

		It("should return false with an old object that is not a service", func() {
			Expect(p.Update(event.UpdateEvent{ObjectOld: &corev1.Node{}, ObjectNew: service})).To(Equal(false))
		})

		It("should return false with a new object that is not a service", func() {
			Expect(p.Update(event.UpdateEvent{ObjectOld: service, ObjectNew: &corev1.Node{}})).To(Equal(false))
		})

		It("should return true if the new service is missing from the service cache (and add it to the service cache)", func() {
			serviceCache.EXPECT().Get(serviceName).Return(nil, false)
			serviceCache.EXPECT().Set(serviceName, projection, azureservice.CacheTTL)

			Expect(p.Update(event.UpdateEvent{ObjectNew: service, ObjectOld: service})).To(Equal(true))
		})

		It("should return true if the deletion timestamp of the new service is different from that of the old service", func() {
			newService := service.DeepCopy()
			newService.DeletionTimestamp = &now
			newProjection := &azureservice.Projection{DeletionTimestamp: &now, LoadBalancerIPs: map[string]bool{ip: true}}
			serviceCache.EXPECT().Get(serviceName).Return(newProjection, true)
			serviceCache.EXPECT().Set(serviceName, newProjection, azureservice.CacheTTL)

			Expect(p.Update(event.UpdateEvent{ObjectNew: newService, ObjectOld: service})).To(Equal(true))
		})

		It("should return true if the deletion timestamp of the new service is different from that of the cached service", func() {
			newService := service.DeepCopy()
			newService.DeletionTimestamp = &now
			newProjection := &azureservice.Projection{DeletionTimestamp: &now, LoadBalancerIPs: map[string]bool{ip: true}}
			serviceCache.EXPECT().Get(serviceName).Return(projection, true)
			serviceCache.EXPECT().Set(serviceName, newProjection, azureservice.CacheTTL)

			Expect(p.Update(event.UpdateEvent{ObjectNew: newService, ObjectOld: newService})).To(Equal(true))
		})

		It("should return true if the ignore annotation of the new service is different from that of the old service", func() {
			newService := service.DeepCopy()
			newService.Annotations = map[string]string{azure.IgnoreAnnotation: "true"}
			newProjection := &azureservice.Projection{ShouldIgnore: true, LoadBalancerIPs: map[string]bool{ip: true}}
			serviceCache.EXPECT().Get(serviceName).Return(newProjection, true)
			serviceCache.EXPECT().Set(serviceName, newProjection, azureservice.CacheTTL)

			Expect(p.Update(event.UpdateEvent{ObjectNew: newService, ObjectOld: service})).To(Equal(true))
		})

		It("should return true if the ignore annotation of the new service is different from that of the cached service", func() {
			newService := service.DeepCopy()
			newService.Annotations = map[string]string{azure.IgnoreAnnotation: "true"}
			newProjection := &azureservice.Projection{ShouldIgnore: true, LoadBalancerIPs: map[string]bool{ip: true}}
			serviceCache.EXPECT().Get(serviceName).Return(projection, true)
			serviceCache.EXPECT().Set(serviceName, newProjection, azureservice.CacheTTL)

			Expect(p.Update(event.UpdateEvent{ObjectNew: newService, ObjectOld: newService})).To(Equal(true))
		})

		It("should return true if the LoadBalancer IPs of the new service are different from that of the old service", func() {
			newService := service.DeepCopy()
			newService.Status.LoadBalancer.Ingress = nil
			newProjection := &azureservice.Projection{LoadBalancerIPs: map[string]bool{}}
			serviceCache.EXPECT().Get(serviceName).Return(newProjection, true)
			serviceCache.EXPECT().Set(serviceName, newProjection, azureservice.CacheTTL)

			Expect(p.Update(event.UpdateEvent{ObjectNew: newService, ObjectOld: service})).To(Equal(true))
		})

		It("should return true if the LoadBalancer IPs the new service are different from that of the cached service", func() {
			newService := service.DeepCopy()
			newService.Status.LoadBalancer.Ingress = nil
			newProjection := &azureservice.Projection{LoadBalancerIPs: map[string]bool{}}
			serviceCache.EXPECT().Get(serviceName).Return(projection, true)
			serviceCache.EXPECT().Set(serviceName, newProjection, azureservice.CacheTTL)

			Expect(p.Update(event.UpdateEvent{ObjectNew: newService, ObjectOld: newService})).To(Equal(true))
		})

		It("should return false if the new service is not different from the old or the cached service", func() {
			serviceCache.EXPECT().Get(serviceName).Return(projection, true)

			Expect(p.Update(event.UpdateEvent{ObjectNew: service, ObjectOld: service})).To(Equal(false))
		})
	})

	Describe("#Delete", func() {
		It("should return false with an empty event", func() {
			Expect(p.Delete(event.DeleteEvent{})).To(Equal(false))
		})

		It("should return false with an object that is not a service", func() {
			Expect(p.Delete(event.DeleteEvent{Object: &corev1.Node{}})).To(Equal(false))
		})

		It("should return true with an object that is a service (and delete it from the service cache)", func() {
			serviceCache.EXPECT().Delete(serviceName)

			Expect(p.Delete(event.DeleteEvent{Object: service})).To(Equal(true))
		})
	})
})
