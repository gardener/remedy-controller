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

	"github.com/gardener/gardener/pkg/controllerutils"
	controllererror "github.com/gardener/gardener/pkg/controllerutils/reconciler"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/runtime/inject"
)

type reconciler struct {
	actuator            Actuator
	controllerName      string
	finalizerName       string
	typ                 client.Object
	shouldEnsureDeleted bool
	client              client.Client
	reader              client.Reader
	logger              logr.Logger
}

// NewReconciler creates a new generic Reconciler.
func NewReconciler(actuator Actuator, controllerName, finalizerName string, typ client.Object, shouldEnsureDeleted bool, logger logr.Logger) reconcile.Reconciler {
	logger.Info("Creating reconciler", "controllerName", controllerName)
	return &reconciler{
		actuator:            actuator,
		controllerName:      controllerName,
		finalizerName:       finalizerName,
		typ:                 typ,
		shouldEnsureDeleted: shouldEnsureDeleted,
		logger:              logger,
	}
}

func (r *reconciler) InjectFunc(f inject.Func) error {
	return f(r.actuator)
}

func (r *reconciler) InjectClient(client client.Client) error {
	r.client = client
	return nil
}

func (r *reconciler) InjectAPIReader(reader client.Reader) error {
	r.reader = reader
	return nil
}

func (r *reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	obj := r.typ.DeepCopyObject().(client.Object)
	obj.SetName(request.Name)
	obj.SetNamespace(request.Namespace)
	logger := r.logger.WithValues("name", obj.GetName(), "namespace", obj.GetNamespace())

	if err := r.client.Get(ctx, request.NamespacedName, obj); err != nil {
		if apierrors.IsNotFound(err) {
			if r.shouldEnsureDeleted {
				return r.ensureDeleted(ctx, obj, logger)
			}
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, errors.Wrap(err, "could not get object")
	}

	switch {
	case obj.GetDeletionTimestamp() != nil:
		return r.delete(ctx, obj, logger)
	default:
		return r.createOrUpdate(ctx, obj, logger)
	}
}

func (r *reconciler) createOrUpdate(ctx context.Context, obj client.Object, logger logr.Logger) (reconcile.Result, error) {
	shouldFinalize, err := r.actuator.ShouldFinalize(ctx, obj)
	if err != nil {
		return reconcile.Result{}, errors.Wrap(err, "could not check if the object should be finalized")
	}
	if shouldFinalize {

		if err := controllerutils.AddFinalizers(ctx, r.client, obj, r.finalizerName); err != nil {
			if apierrors.IsNotFound(err) {
				return reconcile.Result{}, nil
			}
			return reconcile.Result{}, errors.Wrap(err, "could not ensure finalizer")
		}
	} else {
		if !controllerutil.ContainsFinalizer(obj, r.finalizerName) {
			return reconcile.Result{}, nil
		}
	}

	logger.Info("Reconciling object creation or update")
	requeueAfter, err := r.actuator.CreateOrUpdate(ctx, obj)
	if err != nil {
		return reconcileErr(errors.Wrap(err, "could not reconcile object creation or update"))
	}
	logger.Info("Successfully reconciled object creation or update")

	if !shouldFinalize {
		logger.Info("Removing finalizer")
		if err := controllerutils.RemoveFinalizers(ctx, r.client, obj, r.finalizerName); client.IgnoreNotFound(err) != nil {
			return reconcile.Result{}, errors.Wrap(err, "could not remove finalizer")
		}
		return reconcile.Result{}, nil
	}

	if requeueAfter != time.Duration(0) {
		return reconcile.Result{RequeueAfter: requeueAfter}, nil
	}
	return reconcile.Result{}, nil
}

func (r *reconciler) delete(ctx context.Context, obj client.Object, logger logr.Logger) (reconcile.Result, error) {
	if !controllerutil.ContainsFinalizer(obj, r.finalizerName) {
		return reconcile.Result{}, nil
	}

	logger.Info("Reconciling object deletion")
	requeueAfter, err := r.actuator.Delete(ctx, obj)
	if err != nil {
		return reconcileErr(errors.Wrap(err, "could not reconcile object deletion"))
	}
	logger.Info("Successfully reconciled object deletion")

	logger.Info("Removing finalizer")
	if err := controllerutils.RemoveFinalizers(ctx, r.client, obj, r.finalizerName); client.IgnoreNotFound(err) != nil {
		return reconcile.Result{}, errors.Wrap(err, "could not remove finalizer")
	}

	if requeueAfter != time.Duration(0) {
		return reconcile.Result{RequeueAfter: requeueAfter}, nil
	}
	return reconcile.Result{}, nil
}

func (r *reconciler) ensureDeleted(ctx context.Context, obj client.Object, logger logr.Logger) (reconcile.Result, error) {
	logger.Info("Ensuring object deletion")
	if _, err := r.actuator.Delete(ctx, obj); err != nil {
		return reconcileErr(errors.Wrap(err, "could not ensure object deletion"))
	}
	logger.Info("Successfully ensured object deletion")

	return reconcile.Result{}, nil
}

// reconcileErr returns a reconcile.Result or an error, depending on whether the error is a
// RequeueAfterError or not.
func reconcileErr(err error) (reconcile.Result, error) {
	if requeueAfter, ok := errors.Cause(err).(*controllererror.RequeueAfterError); ok {
		return reconcile.Result{Requeue: true, RequeueAfter: requeueAfter.RequeueAfter}, nil
	}
	return reconcile.Result{}, err
}
