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

package service

import (
	"reflect"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/cache"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

const (
	serviceCacheTTL = 10 * time.Hour
)

// NewServicePredicate creates a new predicate that filters only relevant service events,
// such as creating or deleting a service, updating the deletion timestamp of a service with LoadBalancer IPs,
// updating the ignore annotation of a service with LoadBalancer IPs, and updating the service LoadBalancer IPs.
func NewServicePredicate(logger logr.Logger) predicate.Predicate {
	serviceCache := cache.NewExpiring()
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			if e.Object == nil {
				logger.Error(nil, "CreateEvent has no object", "event", e)
				return false
			}
			service, ok := e.Object.(*corev1.Service)
			if !ok {
				return false
			}
			logger := logger.WithValues("name", service.Name, "namespace", service.Namespace)
			logger.Info("Creating a service")
			serviceCache.Set(service.Name, service, serviceCacheTTL)
			return true
		},

		UpdateFunc: func(e event.UpdateEvent) (result bool) {
			if e.ObjectOld == nil || e.ObjectNew == nil {
				logger.Error(nil, "UpdateEvent has no old or new metadata, or no old or new object", "event", e)
				return false
			}
			var oldService, newService, cachedService *corev1.Service
			var ok bool
			if oldService, ok = e.ObjectOld.(*corev1.Service); !ok {
				return false
			}
			if newService, ok = e.ObjectNew.(*corev1.Service); !ok {
				return false
			}
			if v, ok := serviceCache.Get(newService.Name); ok {
				cachedService = v.(*corev1.Service)
			}
			defer func() {
				// In order to prevent lock contention and scalability issues when the cache contains a large number
				// of objects, only update the cache if we detected a change we are interested in
				// We can avoid updating the cache on other changes since they won't affect subsequent comparisons
				// with the cached object
				if result {
					serviceCache.Set(newService.Name, newService, serviceCacheTTL)
				}
			}()
			logger := logger.WithValues("name", newService.Name, "namespace", newService.Namespace)
			if cachedService == nil {
				logger.Info("Updating a service that is missing in the service cache")
				return true
			}
			oldIPs, newIPs, cachedIPs := getServiceLoadBalancerIPs(oldService), getServiceLoadBalancerIPs(newService), getServiceLoadBalancerIPs(cachedService)
			if len(newIPs) > 0 && (newService.DeletionTimestamp != oldService.DeletionTimestamp || newService.DeletionTimestamp != cachedService.DeletionTimestamp) {
				logger.Info("Updating the deletion timestamp of a service with LoadBalancer IPs")
				return true
			}
			if len(newIPs) > 0 && (shouldIgnoreService(newService) != shouldIgnoreService(oldService) || shouldIgnoreService(newService) != shouldIgnoreService(cachedService)) {
				logger.Info("Updating the ignore annotation of a service with LoadBalancer IPs")
				return true
			}
			if !reflect.DeepEqual(newIPs, oldIPs) || !reflect.DeepEqual(newIPs, cachedIPs) {
				logger.Info("Updating service LoadBalancer IPs")
				return true
			}
			return false
		},

		DeleteFunc: func(e event.DeleteEvent) bool {
			if e.Object == nil {
				logger.Error(nil, "DeleteEvent has no object", "event", e)
				return false
			}
			service, ok := e.Object.(*corev1.Service)
			if !ok {
				return false
			}
			logger := logger.WithValues("name", service.Name, "namespace", service.Namespace)
			logger.Info("Deleting a service")
			serviceCache.Delete(service.Name)
			return true
		},

		GenericFunc: func(e event.GenericEvent) bool {
			return false
		},
	}
}
