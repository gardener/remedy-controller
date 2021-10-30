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

package controller

import (
	"context"
	"time"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/pkg/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// Actuator acts upon objects being reconciled by a Reconciler.
type Actuator interface {
	// CreateOrUpdate reconciles object creation or update.
	CreateOrUpdate(context.Context, client.Object) (time.Duration, error)
	// Delete reconciles object deletion.
	Delete(context.Context, client.Object) (time.Duration, error)
	// ShouldFinalize returns true if the object should be finalized.
	ShouldFinalize(context.Context, client.Object) (bool, error)
}

// AddArgs are arguments for adding a controller to a manager.
type AddArgs struct {
	// Actuator is an actuator.
	Actuator Actuator
	// ControllerName is the name of the controller.
	ControllerName string
	// FinalizerName is the finalizer name.
	FinalizerName string
	// ControllerOptions are the controller options to use when creating a controller.
	// The Reconciler field is always overridden with a reconciler created from the given actuator.
	ControllerOptions controller.Options
	// Type is the object type to watch.
	Type client.Object
	// ShouldEnsureDeleted specifies that the controller should ensure the object is properly deleted on not found.
	ShouldEnsureDeleted bool
	// Predicates are the predicates to use when watching objects.
	Predicates []predicate.Predicate
	// WatchBuilder defines additional watches that should be set up.
	WatchBuilder extensionscontroller.WatchBuilder
}

// DefaultPredicates returns the default predicates for a reconciler.
func DefaultPredicates() []predicate.Predicate {
	return []predicate.Predicate{}
}

// Add creates a new controller and adds it to the given manager using the given args.
func Add(mgr manager.Manager, args AddArgs) error {
	args.ControllerOptions.Reconciler = NewReconciler(args.Actuator, args.ControllerName, args.FinalizerName, args.Type, args.ShouldEnsureDeleted, log.Log.WithName(args.ControllerName))
	return add(mgr, args)
}

func add(mgr manager.Manager, args AddArgs) error {
	// Create controller
	ctrl, err := controller.New(args.ControllerName, mgr, args.ControllerOptions)
	if err != nil {
		return errors.Wrap(err, "could not create controller")
	}

	// Add primary watch
	if err := ctrl.Watch(&source.Kind{Type: args.Type}, &handler.EnqueueRequestForObject{}, args.Predicates...); err != nil {
		return errors.Wrap(err, "could not setup primary watch")
	}

	// Add additional watches to the controller besides the primary one.
	if err = args.WatchBuilder.AddToController(ctrl); err != nil {
		return errors.Wrap(err, "could not setup additional watches")
	}

	return nil
}
