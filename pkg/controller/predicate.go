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

package controller

import (
	"context"

	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// NewOwnedObjectPredicate creates a new predicate that filters only relevant owned object events,
// such as creating or updating an object without an owner or with an owner that is being deleted,
// or deleting an object with an owner that is not being deleted.
func NewOwnedObjectPredicate(ownerType client.Object, reader client.Reader, ownerMapper Mapper, finalizer string, logger logr.Logger) predicate.Predicate {
	return &ownedObjectPredicate{
		ownerType:   ownerType,
		reader:      reader,
		ownerMapper: ownerMapper,
		finalizer:   finalizer,
		logger:      logger,
	}
}

type ownedObjectPredicate struct {
	reader      client.Reader
	ownerType   client.Object
	ownerMapper Mapper
	finalizer   string
	logger      logr.Logger
}

// Create returns true if the Create event should be processed.
func (p *ownedObjectPredicate) Create(e event.CreateEvent) bool {
	if e.Object == nil {
		p.logger.Error(nil, "CreateEvent has no object", "event", e)
		return false
	}
	return p.handleCreateOrUpdateEvent(e.Object)
}

// Update returns true if the Update event should be processed.
func (p *ownedObjectPredicate) Update(e event.UpdateEvent) bool {
	if e.ObjectNew == nil {
		p.logger.Error(nil, "UpdateEvent has no new object", "event", e)
		return false
	}
	return p.handleCreateOrUpdateEvent(e.ObjectNew)
}

// Delete returns true if the Delete event should be processed.
func (p *ownedObjectPredicate) Delete(e event.DeleteEvent) bool {
	if e.Object == nil {
		p.logger.Error(nil, "DeleteEvent has no object", "event", e)
		return false
	}
	return p.handleDeleteEvent(e.Object)
}

// Generic returns true if the Generic event should be processed.
func (p *ownedObjectPredicate) Generic(event.GenericEvent) bool {
	return false
}

func (p *ownedObjectPredicate) handleCreateOrUpdateEvent(obj client.Object) bool {
	ownerKey := p.ownerMapper.Map(obj)
	if ownerKey.Name == "" {
		return false
	}
	logger := p.logger.WithValues("name", obj.GetName(), "namespace", obj.GetNamespace(), "ownerName", ownerKey.Name, "ownerNamespace", ownerKey.Namespace)
	owner, err := p.getOwner(ownerKey)
	if err != nil {
		return false
	}
	if owner == nil {
		logger.Info("Creating or updating an object without an owner")
		return true
	}
	if !controllerutil.ContainsFinalizer(owner, p.finalizer) {
		return false
	}
	if obj.GetDeletionTimestamp() == nil && owner.GetDeletionTimestamp() != nil {
		logger.Info("Creating or updating an object with an owner that is being deleted")
		return true
	}
	return false
}

func (p *ownedObjectPredicate) handleDeleteEvent(obj client.Object) bool {
	ownerKey := p.ownerMapper.Map(obj)
	if ownerKey.Name == "" {
		return false
	}
	logger := p.logger.WithValues("name", obj.GetName(), "namespace", obj.GetNamespace(), "ownerName", ownerKey.Name, "ownerNamespace", ownerKey.Namespace)
	owner, err := p.getOwner(ownerKey)
	if err != nil {
		return false
	}
	if owner == nil || !controllerutil.ContainsFinalizer(owner, p.finalizer) {
		return false
	}
	if owner.GetDeletionTimestamp() == nil {
		logger.Info("Deleting an object with an owner that is not being deleted")
		return true
	}
	return false
}

func (p *ownedObjectPredicate) getOwner(key client.ObjectKey) (client.Object, error) {
	obj := p.ownerType.DeepCopyObject().(client.Object)
	if err := p.reader.Get(context.Background(), key, obj); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return obj, nil
}
