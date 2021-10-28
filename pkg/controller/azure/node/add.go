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
	remedycontroller "github.com/gardener/remedy-controller/pkg/controller"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

const (
	// ControllerName is the name of the Azure node controller.
	ControllerName = "azurenode-controller"
	// ActuatorName is the name of the Azure service actuator.
	ActuatorName = "azurenode-actuator"
	// PredicateName is the name of the predicate of the Azure node controller.
	PredicateName = "azurenode-ready-unreachable-changed-predicate"
	// FinalizerName is the finalizer to put on node resources.
	FinalizerName = "azure.remedy.gardener.cloud/node"
)

var (
	// DefaultAddOptions are the default AddOptions for AddToManager.
	DefaultAddOptions = AddOptions{}
)

// AddOptions are options to apply when adding a controller to a manager.
type AddOptions struct {
	// Controller are the controller.Options.
	Controller controller.Options
	// Client is the Kubernetes client for the control cluster.
	Client client.Client
	// Namespace is the namespace for custom resources in the control cluster.
	Namespace string
}

// AddToManagerWithOptions adds a controller with the given AddOptions to the given manager.
func AddToManagerWithOptions(mgr manager.Manager, options AddOptions) error {
	return remedycontroller.Add(mgr, remedycontroller.AddArgs{
		Actuator:          NewActuator(options.Client, options.Namespace, log.Log.WithName(ActuatorName)),
		ControllerName:    ControllerName,
		FinalizerName:     FinalizerName,
		ControllerOptions: options.Controller,
		Type:              &corev1.Node{},
		Predicates: []predicate.Predicate{
			NewReadyUnreachableChangedPredicate(log.Log.WithName(PredicateName)),
		},
	})
}

// AddToManager adds a controller with the default AddOptions to the given manager.
func AddToManager(mgr manager.Manager) error {
	return AddToManagerWithOptions(mgr, DefaultAddOptions)
}
