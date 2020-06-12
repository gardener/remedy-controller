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

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

func NewLoadBalancerIPsChangedPredicate() predicate.Predicate {
	logger := log.Log.WithName("load-balancer-ips-changed-predicate")
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			if e.Meta == nil || e.Object == nil {
				logger.Error(nil, "CreateEvent has no metadata, or no object", "event", e)
				return false
			}
			var service *corev1.Service
			var ok bool
			if service, ok = e.Object.(*corev1.Service); !ok {
				return false
			}
			ips := getServiceLoadBalancerIPs(service)
			if len(ips) > 0 {
				logger.Info("Creating a service with LoadBalancer IPs")
				return true
			}
			return false
		},

		UpdateFunc: func(e event.UpdateEvent) bool {
			if e.MetaOld == nil || e.MetaNew == nil || e.ObjectOld == nil || e.ObjectNew == nil {
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
			if len(newIPs) > 0 && e.MetaOld.GetDeletionTimestamp() != e.MetaNew.GetDeletionTimestamp() {
				logger.Info("Updating the deletion timestamp of a service with LoadBalancer IPs")
				return true
			}
			if !reflect.DeepEqual(oldIPs, newIPs) {
				logger.Info("Updating service LoadBalancer IPs")
				return true
			}
			return false
		},

		DeleteFunc: func(e event.DeleteEvent) bool {
			if e.Meta == nil || e.Object == nil {
				logger.Error(nil, "DeleteEvent has no metadata, or no object", "event", e)
				return false
			}
			var service *corev1.Service
			var ok bool
			if service, ok = e.Object.(*corev1.Service); !ok {
				return false
			}
			ips := getServiceLoadBalancerIPs(service)
			if len(ips) > 0 {
				logger.Info("Deleting a service with LoadBalancer IPs")
				return true
			}
			return false
		},

		GenericFunc: func(e event.GenericEvent) bool {
			return false
		},
	}
}
