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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/gardener/remedy-controller/pkg/utils"
)

const (
	// CacheTTL is the TTL for service cache entries.
	CacheTTL = 10 * time.Hour
)

// NewPredicate creates a new predicate that filters only relevant service events,
// such as creating or deleting a service, updating the deletion timestamp of a service with LoadBalancer IPs,
// updating the ignore annotation of a service with LoadBalancer IPs, and updating the service LoadBalancer IPs.
func NewPredicate(serviceCache utils.ExpiringCache, logger logr.Logger) predicate.Predicate {
	return &servicePredicate{
		serviceCache: serviceCache,
		logger:       logger,
	}
}

type servicePredicate struct {
	serviceCache utils.ExpiringCache
	logger       logr.Logger
}

// Create returns true if the Create event should be processed.
func (p *servicePredicate) Create(e event.CreateEvent) bool {
	if e.Object == nil {
		p.logger.Error(nil, "CreateEvent has no object", "event", e)
		return false
	}
	service, ok := e.Object.(*corev1.Service)
	if !ok {
		return false
	}
	logger := p.logger.WithValues("name", service.Name, "namespace", service.Namespace)
	logger.Info("Creating a service")
	p.serviceCache.Set(service.Name, NewProjection(service), CacheTTL)
	return true
}

// Update returns true if the Update event should be processed.
func (p *servicePredicate) Update(e event.UpdateEvent) (result bool) {
	if e.ObjectOld == nil || e.ObjectNew == nil {
		p.logger.Error(nil, "UpdateEvent has no old or new metadata, or no old or new object", "event", e)
		return false
	}
	var oldService, newService *corev1.Service
	var ok bool
	if oldService, ok = e.ObjectOld.(*corev1.Service); !ok {
		return false
	}
	if newService, ok = e.ObjectNew.(*corev1.Service); !ok {
		return false
	}
	var cachedService *Projection
	if v, ok := p.serviceCache.Get(newService.Name); ok {
		cachedService = v.(*Projection)
	}
	defer func() {
		// In order to prevent lock contention and scalability issues when the cache contains a large number
		// of objects, only update the cache if we detected a change we are interested in
		// We can avoid updating the cache on other changes since they won't affect subsequent comparisons
		// with the cached object
		if result {
			p.serviceCache.Set(newService.Name, NewProjection(newService), CacheTTL)
		}
	}()
	logger := p.logger.WithValues("name", newService.Name, "namespace", newService.Namespace)
	if cachedService == nil {
		logger.Info("Updating a service that is missing in the service cache")
		return true
	}
	oldIPs, newIPs := getServiceLoadBalancerIPs(oldService), getServiceLoadBalancerIPs(newService)
	if len(newIPs) > 0 && (newService.DeletionTimestamp != oldService.DeletionTimestamp || newService.DeletionTimestamp != cachedService.DeletionTimestamp) {
		logger.Info("Updating the deletion timestamp of a service with LoadBalancer IPs")
		return true
	}
	if len(newIPs) > 0 && (shouldIgnoreService(newService) != shouldIgnoreService(oldService) || shouldIgnoreService(newService) != cachedService.ShouldIgnore) {
		logger.Info("Updating the ignore annotation of a service with LoadBalancer IPs")
		return true
	}
	if !reflect.DeepEqual(newIPs, oldIPs) || !reflect.DeepEqual(newIPs, cachedService.LoadBalancerIPs) {
		logger.Info("Updating service LoadBalancer IPs")
		return true
	}
	return false
}

// Delete returns true if the Delete event should be processed.
func (p *servicePredicate) Delete(e event.DeleteEvent) bool {
	if e.Object == nil {
		p.logger.Error(nil, "DeleteEvent has no object", "event", e)
		return false
	}
	service, ok := e.Object.(*corev1.Service)
	if !ok {
		return false
	}
	logger := p.logger.WithValues("name", service.Name, "namespace", service.Namespace)
	logger.Info("Deleting a service")
	p.serviceCache.Delete(service.Name)
	return true
}

// Generic returns true if the Generic event should be processed.
func (p *servicePredicate) Generic(_ event.GenericEvent) bool {
	return false
}

// Projection captures only the essential properties of a service that is being cached.
// By using projections, we prevent the cache from getting too big for clusters with large number of services.
type Projection struct {
	DeletionTimestamp *metav1.Time
	ShouldIgnore      bool
	LoadBalancerIPs   map[string]bool
}

// NewProjection creates a new Projection from the given service.
func NewProjection(service *corev1.Service) *Projection {
	return &Projection{
		DeletionTimestamp: service.DeletionTimestamp,
		ShouldIgnore:      shouldIgnoreService(service),
		LoadBalancerIPs:   getServiceLoadBalancerIPs(service),
	}
}
