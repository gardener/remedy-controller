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
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// NewReadyUnreachableChangedPredicate creates a new predicate that filters only relevant node events,
// such as node creation and deletion, updating the deletion timestamp of a node, changes to the "Ready" condition,
// and changes to the "Unreachable" taint.
func NewReadyUnreachableChangedPredicate(logger logr.Logger) predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			if e.Object == nil {
				logger.Error(nil, "CreateEvent has no object", "event", e)
				return false
			}
			if _, ok := e.Object.(*corev1.Node); !ok {
				return false
			}
			logger.Info("Creating a node")
			return true
		},

		UpdateFunc: func(e event.UpdateEvent) bool {
			if e.ObjectOld == nil || e.ObjectNew == nil {
				logger.Error(nil, "UpdateEvent has no old or new metadata, or no old or new object", "event", e)
				return false
			}
			var oldNode, newNode *corev1.Node
			var ok bool
			if oldNode, ok = e.ObjectOld.(*corev1.Node); !ok {
				return false
			}
			if newNode, ok = e.ObjectNew.(*corev1.Node); !ok {
				return false
			}
			if e.ObjectOld.GetDeletionTimestamp() != e.ObjectNew.GetDeletionTimestamp() {
				logger.Info("Updating the deletion timestamp of a node")
				return true
			}
			if isNodeReady(oldNode) != isNodeReady(newNode) || isNodeUnreachable(oldNode) != isNodeUnreachable(newNode) {
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
			if _, ok := e.Object.(*corev1.Node); !ok {
				return false
			}
			logger.Info("Deleting a node")
			return true
		},

		GenericFunc: func(e event.GenericEvent) bool {
			return false
		},
	}
}
