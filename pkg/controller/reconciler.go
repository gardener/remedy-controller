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
	controllererror "github.com/gardener/gardener/extensions/pkg/controller/error"
	"github.com/gardener/gardener/extensions/pkg/util"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/runtime/inject"
)

type reconciler struct {
	actuator       Actuator
	controllerName string
	finalizerName  string
	typ            runtime.Object
	ctx            context.Context
	client         client.Client
	logger         logr.Logger
}

// NewReconciler creates a new generic Reconciler.
func NewReconciler(mgr manager.Manager, actuator Actuator, controllerName, finalizerName string, typ runtime.Object, logger logr.Logger) reconcile.Reconciler {
	logger.Info("Creating reconciler", "controllerName", controllerName)
	return &reconciler{
		actuator:       actuator,
		controllerName: controllerName,
		finalizerName:  finalizerName,
		typ:            typ,
		logger:         logger,
	}
}

func (r *reconciler) InjectFunc(f inject.Func) error {
	return f(r.actuator)
}

func (r *reconciler) InjectClient(client client.Client) error {
	r.client = client
	return nil
}

func (r *reconciler) InjectStopChannel(stopCh <-chan struct{}) error {
	r.ctx = util.ContextFromStopChannel(stopCh)
	return nil
}

func (r *reconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	obj := r.typ.DeepCopyObject()
	if err := r.client.Get(r.ctx, request.NamespacedName, obj); err != nil {
		if apierrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, errors.Wrap(err, "could not get object")
	}

	accessor, err := meta.Accessor(obj)
	if err != nil {
		return reconcile.Result{}, errors.Wrap(err, "could not get object accessor")
	}

	logger := r.logger.WithValues("kind", obj.GetObjectKind().GroupVersionKind(), "name", accessor.GetName(), "namespace", accessor.GetNamespace())

	switch {
	case accessor.GetDeletionTimestamp() != nil:
		return r.delete(r.ctx, obj, logger)
	default:
		return r.createOrUpdate(r.ctx, obj, logger)
	}
}

func (r *reconciler) createOrUpdate(ctx context.Context, obj runtime.Object, logger logr.Logger) (reconcile.Result, error) {
	shouldFinalize, err := r.actuator.ShouldFinalize(ctx, obj)
	if err != nil {
		return reconcile.Result{}, errors.Wrap(err, "could not check if the object should be finalized")
	}
	if shouldFinalize {
		if err := extensionscontroller.EnsureFinalizer(ctx, r.client, r.finalizerName, obj); err != nil {
			return reconcile.Result{}, errors.Wrap(err, "could not ensure finalizer")
		}
	} else {
		hasFinalizer, err := extensionscontroller.HasFinalizer(obj, r.finalizerName)
		if err != nil {
			return reconcile.Result{}, errors.Wrap(err, "could not check for finalizer")
		}
		if !hasFinalizer {
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
		if err := extensionscontroller.DeleteFinalizer(ctx, r.client, r.finalizerName, obj); err != nil {
			return reconcile.Result{}, errors.Wrap(err, "could not remove finalizer")
		}
	}

	if requeueAfter != time.Duration(0) {
		return reconcile.Result{RequeueAfter: requeueAfter}, nil
	}
	return reconcile.Result{}, nil
}

func (r *reconciler) delete(ctx context.Context, obj runtime.Object, logger logr.Logger) (reconcile.Result, error) {
	hasFinalizer, err := extensionscontroller.HasFinalizer(obj, r.finalizerName)
	if err != nil {
		return reconcile.Result{}, errors.Wrap(err, "could not check for finalizer")
	}
	if !hasFinalizer {
		return reconcile.Result{}, nil
	}

	logger.Info("Reconciling object deletion")
	if err := r.actuator.Delete(r.ctx, obj); err != nil {
		return reconcileErr(errors.Wrap(err, "could not reconcile object deletion"))
	}
	logger.Info("Successfully reconciled object deletion")

	logger.Info("Removing finalizer")
	if err := extensionscontroller.DeleteFinalizer(ctx, r.client, r.finalizerName, obj); err != nil {
		return reconcile.Result{}, errors.Wrap(err, "could not remove finalizer")
	}

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
