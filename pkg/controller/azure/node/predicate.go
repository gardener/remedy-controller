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

package node

import (
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/cache"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

const (
	nodeCacheTTL = 10 * time.Hour
)

// NewNodePredicate creates a new predicate that filters only relevant node events,
// such as creating or deleting a node, updating the deletion timestamp of a node,
// or updating the ready condition or unreachable taint of a node.
func NewNodePredicate(logger logr.Logger) predicate.Predicate {
	nodeCache := cache.NewExpiring()
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			if e.Object == nil {
				logger.Error(nil, "CreateEvent has no object", "event", e)
				return false
			}
			node, ok := e.Object.(*corev1.Node)
			if !ok {
				return false
			}
			logger := logger.WithValues("name", node.Name)
			logger.Info("Creating a node")
			nodeCache.Set(node.Name, node, nodeCacheTTL)
			return true
		},

		UpdateFunc: func(e event.UpdateEvent) (result bool) {
			if e.ObjectOld == nil || e.ObjectNew == nil {
				logger.Error(nil, "UpdateEvent has no no old or new object", "event", e)
				return false
			}
			var oldNode, newNode, cachedNode *corev1.Node
			var ok bool
			if oldNode, ok = e.ObjectOld.(*corev1.Node); !ok {
				return false
			}
			if newNode, ok = e.ObjectNew.(*corev1.Node); !ok {
				return false
			}
			if v, ok := nodeCache.Get(newNode.Name); ok {
				cachedNode = v.(*corev1.Node)
			}
			defer func() {
				// In order to prevent lock contention and scalability issues when the cache contains a large number
				// of objects, only update the cache if we detected a change we are interested in
				// We can avoid updating the cache on other changes since they won't affect subsequent comparisons
				// with the cached object
				if result {
					nodeCache.Set(newNode.Name, newNode, nodeCacheTTL)
				}
			}()
			logger := logger.WithValues("name", newNode.Name)
			if cachedNode == nil {
				logger.Info("Updating a node that is missing in the node cache")
				return true
			}
			if newNode.DeletionTimestamp != oldNode.DeletionTimestamp || newNode.DeletionTimestamp != cachedNode.DeletionTimestamp {
				logger.Info("Updating the deletion timestamp of a node")
				return true
			}
			if isNodeNotReadyOrUnreachable(newNode) != isNodeNotReadyOrUnreachable(oldNode) || isNodeNotReadyOrUnreachable(newNode) != isNodeNotReadyOrUnreachable(cachedNode) {
				logger.Info("Updating the ready condition or unreachable taint of a node")
				return true
			}
			return false
		},

		DeleteFunc: func(e event.DeleteEvent) bool {
			if e.Object == nil {
				logger.Error(nil, "DeleteEvent has no object", "event", e)
				return false
			}
			node, ok := e.Object.(*corev1.Node)
			if !ok {
				return false
			}
			logger := logger.WithValues("name", node.Name)
			logger.Info("Deleting a node")
			nodeCache.Delete(node.Name)
			return true
		},

		GenericFunc: func(e event.GenericEvent) bool {
			return false
		},
	}
}
