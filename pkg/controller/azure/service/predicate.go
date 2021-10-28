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

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// NewLoadBalancerIPsChangedPredicate creates a new predicate that filters only relevant service events,
// such as service creation and deletion, updating the deletion timestamp of a service with LoadBalancer IPs,
// and changes to the LandBalancer IPs of a service.
func NewLoadBalancerIPsChangedPredicate(logger logr.Logger) predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			if e.Object == nil {
				logger.Error(nil, "CreateEvent has no object", "event", e)
				return false
			}
			if _, ok := e.Object.(*corev1.Service); !ok {
				return false
			}
			logger.Info("Creating a service")
			return true
		},

		UpdateFunc: func(e event.UpdateEvent) bool {
			if e.ObjectOld == nil || e.ObjectNew == nil {
				logger.Error(nil, "UpdateEvent has no old or new metadata, or no old or new object", "event", e)
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
			oldIPs, newIPs := getServiceLoadBalancerIPs(oldService), getServiceLoadBalancerIPs(newService)
			if len(newIPs) > 0 && e.ObjectOld.GetDeletionTimestamp() != e.ObjectNew.GetDeletionTimestamp() {
				logger.Info("Updating the deletion timestamp of a service with LoadBalancer IPs")
				return true
			}
			if len(newIPs) > 0 && shouldIgnoreService(oldService) != shouldIgnoreService(newService) {
				logger.Info("Updating the ignore annotation of a service with LoadBalancer IPs")
				return true
			}
			if !reflect.DeepEqual(oldIPs, newIPs) {
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
			if _, ok := e.Object.(*corev1.Service); !ok {
				return false
			}
			logger.Info("Deleting a service")
			return true
		},

		GenericFunc: func(e event.GenericEvent) bool {
			return false
		},
	}
}
