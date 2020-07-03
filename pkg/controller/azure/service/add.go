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
	"github.com/gardener/remedy-controller/pkg/apis/config"
	remedycontroller "github.com/gardener/remedy-controller/pkg/controller"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

const (
	ControllerName = "azureservice-controller"
	ActuatorName   = "azureservice-actuator"
	PredicateName  = "azureservice-load-balancer-ips-changed-predicate"
	FinalizerName  = "azure.remedy.gardener.cloud/service"
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
	// Config is the configuration for the Azure orphaned public IP remedy.
	Config config.AzureOrphanedPublicIPRemedyConfiguration
}

// AddToManagerWithOptions adds a controller with the given AddOptions to the given manager.
func AddToManagerWithOptions(mgr manager.Manager, options AddOptions) error {
	return remedycontroller.Add(mgr, remedycontroller.AddArgs{
		Actuator:          NewActuator(options.Client, options.Config, options.Namespace, log.Log.WithName(ActuatorName)),
		ControllerName:    ControllerName,
		FinalizerName:     FinalizerName,
		ControllerOptions: options.Controller,
		Type:              &corev1.Service{},
		Predicates: []predicate.Predicate{
			NewLoadBalancerIPsChangedPredicate(log.Log.WithName(PredicateName)),
		},
	})
}

// AddToManager adds a controller with the default AddOptions to the given manager.
func AddToManager(mgr manager.Manager) error {
	return AddToManagerWithOptions(mgr, DefaultAddOptions)
}
