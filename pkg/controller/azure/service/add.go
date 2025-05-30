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
	"context"
	"time"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"

	azurev1alpha1 "github.com/gardener/remedy-controller/pkg/apis/azure/v1alpha1"
	"github.com/gardener/remedy-controller/pkg/apis/config"
	remedycontroller "github.com/gardener/remedy-controller/pkg/controller"
	"github.com/gardener/remedy-controller/pkg/controller/azure"
)

const (
	// ControllerName is the name of the Azure service controller.
	ControllerName = "azureservice-controller"
	// ActuatorName is the name of the Azure service actuator.
	ActuatorName = "azureservice-actuator"
	// PredicateName is the name of the predicate of the Azure service controller.
	PredicateName = "azureservice-predicate"
	// PublicIPAddressPredicateName is the name of the predicate of the Azure service controller for filtering publicipaddress events.
	PublicIPAddressPredicateName = "azureservice-publicipaddress-predicate"
	// FinalizerName is the finalizer to put on service resources.
	FinalizerName = "azure.remedy.gardener.cloud/service"
)

var (
	// DefaultAddOptions are the default AddOptions for AddToManager.
	DefaultAddOptions = AddOptions{
		Config: config.AzureOrphanedPublicIPRemedyConfiguration{
			ServiceSyncPeriod: metav1.Duration{Duration: 4 * time.Hour},
		},
	}

	// ObjectLabeler is used to label publicipaddress objects created by this controller.
	ObjectLabeler = remedycontroller.NewNamespacedObjectLabeler(".")
)

// AddOptions are options to apply when adding a controller to a manager.
type AddOptions struct {
	// Controller are the controller.Options.
	Controller controller.Options
	// Client is the Kubernetes client for the control cluster.
	Client client.Client
	// Namespace is the namespace for custom resources in the control cluster.
	Namespace string
	// Manager is the control cluster manager.
	Manager manager.Manager
	// Config is the configuration for the Azure orphaned public IP remedy.
	Config config.AzureOrphanedPublicIPRemedyConfiguration
}

// AddToManagerWithOptions adds a controller with the given AddOptions to the given manager.
func AddToManagerWithOptions(mgr manager.Manager, options AddOptions) error {
	return remedycontroller.Add(mgr, remedycontroller.AddArgs{
		Actuator:            NewActuator(options.Client, options.Namespace, options.Config.ServiceSyncPeriod.Duration, log.Log.WithName(ActuatorName)),
		ControllerName:      ControllerName,
		FinalizerName:       FinalizerName,
		ControllerOptions:   options.Controller,
		Type:                &corev1.Service{},
		ShouldEnsureDeleted: true,
		Predicates: []predicate.Predicate{
			NewPredicate(cache.NewExpiring(), log.Log.WithName(PredicateName)),
		},
		WatchBuilder: extensionscontroller.NewWatchBuilder(func(ctrl controller.Controller) error {
			serviceMapper := remedycontroller.NewLabelMapper(ObjectLabeler, azure.ServiceLabel)
			return ctrl.Watch(
				source.Kind[client.Object](options.Manager.GetCache(),
					&azurev1alpha1.PublicIPAddress{},
					handler.TypedEnqueueRequestsFromMapFunc(remedycontroller.MapFuncFromMapper(serviceMapper)),
					remedycontroller.NewOwnedObjectPredicate(&corev1.Service{}, mgr.GetCache(), serviceMapper, FinalizerName, log.Log.WithName(PublicIPAddressPredicateName)),
				),
			)
		}),
	})
}

// AddToManager adds a controller with the default AddOptions to the given manager.
func AddToManager(_ context.Context, mgr manager.Manager) error {
	return AddToManagerWithOptions(mgr, DefaultAddOptions)
}
