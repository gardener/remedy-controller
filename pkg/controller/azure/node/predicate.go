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

	"github.com/gardener/remedy-controller/pkg/utils"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

const (
	// CacheTTL is the TTL for node cache entries.
	CacheTTL = 10 * time.Hour
)

// NewPredicate creates a new predicate that filters only relevant node events,
// such as creating or deleting a node, updating the deletion timestamp of a node,
// or updating the ready condition or unreachable taint of a node.
func NewPredicate(nodeCache utils.ExpiringCache, logger logr.Logger) predicate.Predicate {
	return &nodePredicate{
		nodeCache: nodeCache,
		logger:    logger,
	}
}

type nodePredicate struct {
	nodeCache utils.ExpiringCache
	logger    logr.Logger
}

// Create returns true if the Create event should be processed.
func (p *nodePredicate) Create(e event.CreateEvent) bool {
	if e.Object == nil {
		p.logger.Error(nil, "CreateEvent has no object", "event", e)
		return false
	}
	node, ok := e.Object.(*corev1.Node)
	if !ok {
		return false
	}
	logger := p.logger.WithValues("name", node.Name)
	logger.Info("Creating a node")
	p.nodeCache.Set(node.Name, NewProjection(node), CacheTTL)
	return true
}

// Update returns true if the Update event should be processed.
func (p *nodePredicate) Update(e event.UpdateEvent) (result bool) {
	if e.ObjectOld == nil || e.ObjectNew == nil {
		p.logger.Error(nil, "UpdateEvent has no no old or new object", "event", e)
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
	var cachedNode *Projection
	if v, ok := p.nodeCache.Get(newNode.Name); ok {
		cachedNode = v.(*Projection)
	}
	defer func() {
		// In order to prevent lock contention and scalability issues when the cache contains a large number
		// of objects, only update the cache if we detected a change we are interested in
		// We can avoid updating the cache on other changes since they won't affect subsequent comparisons
		// with the cached object
		if result {
			p.nodeCache.Set(newNode.Name, NewProjection(newNode), CacheTTL)
		}
	}()
	logger := p.logger.WithValues("name", newNode.Name)
	if cachedNode == nil {
		logger.Info("Updating a node that is missing in the node cache")
		return true
	}
	if newNode.DeletionTimestamp != oldNode.DeletionTimestamp || newNode.DeletionTimestamp != cachedNode.DeletionTimestamp {
		logger.Info("Updating the deletion timestamp of a node")
		return true
	}
	if isNodeNotReadyOrUnreachable(newNode) != isNodeNotReadyOrUnreachable(oldNode) || isNodeNotReadyOrUnreachable(newNode) != cachedNode.NotReadyOrUnreachable {
		logger.Info("Updating the ready condition or unreachable taint of a node")
		return true
	}
	return false
}

// Delete returns true if the Delete event should be processed.
func (p *nodePredicate) Delete(e event.DeleteEvent) bool {
	if e.Object == nil {
		p.logger.Error(nil, "DeleteEvent has no object", "event", e)
		return false
	}
	node, ok := e.Object.(*corev1.Node)
	if !ok {
		return false
	}
	logger := p.logger.WithValues("name", node.Name)
	logger.Info("Deleting a node")
	p.nodeCache.Delete(node.Name)
	return true
}

// Generic returns true if the Generic event should be processed.
func (p *nodePredicate) Generic(_ event.GenericEvent) bool {
	return false
}

// Projection captures only the essential properties of a node that is being cached.
// By using projections, we prevent the cache from getting too big for clusters with large number of nodes.
type Projection struct {
	DeletionTimestamp     *metav1.Time
	NotReadyOrUnreachable bool
}

// NewProjection creates a new Projection from the given node.
func NewProjection(node *corev1.Node) *Projection {
	return &Projection{
		DeletionTimestamp:     node.DeletionTimestamp,
		NotReadyOrUnreachable: isNodeNotReadyOrUnreachable(node),
	}
}
