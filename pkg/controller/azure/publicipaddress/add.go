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

package publicipaddress

import (
	"time"

	azurev1alpha1 "github.com/gardener/remedy-controller/pkg/apis/azure/v1alpha1"
	"github.com/gardener/remedy-controller/pkg/apis/config"
	"github.com/gardener/remedy-controller/pkg/client/azure"
	remedycontroller "github.com/gardener/remedy-controller/pkg/controller"
	"github.com/gardener/remedy-controller/pkg/utils"
	utilsazure "github.com/gardener/remedy-controller/pkg/utils/azure"

	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

const (
	ControllerName = "azurepublicipaddress-controller"
	ActuatorName   = "azurepublicipaddress-actuator"
	FinalizerName  = "azure.remedy.gardener.cloud/publicipaddress"
)

var (
	// DefaultAddOptions are the default AddOptions for AddToManager.
	DefaultAddOptions = AddOptions{
		Config: config.AzureOrphanedPublicIPRemedyConfiguration{
			RequeueInterval:     metav1.Duration{Duration: 1 * time.Minute},
			SyncPeriod:          metav1.Duration{Duration: 10 * time.Hour},
			DeletionGracePeriod: metav1.Duration{Duration: 5 * time.Minute},
			MaxGetAttempts:      5,
			MaxCleanAttempts:    5,
		},
	}

	// CleanedIPsCounter is a global counter for cleaned Azure public IP addresses.
	CleanedIPsCounter = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "cleaned_azure_public_ips_total",
			Help: "Number of cleaned Azure public IPs",
		},
	)
)

// AddOptions are options to apply when adding a controller to a manager.
type AddOptions struct {
	// Controller are the controller.Options.
	Controller controller.Options
	// InfraConfigPath is the path to the infrastructure configuration file.
	InfraConfigPath string
	// Config is the configuration for the Azure orphaned public IP remedy.
	Config config.AzureOrphanedPublicIPRemedyConfiguration
}

// AddToManagerWithOptions adds a controller with the given AddOptions to the given manager.
func AddToManagerWithOptions(mgr manager.Manager, options AddOptions) error {
	// Read Azure credentials from infrastructure config file
	credentials, err := azure.ReadConfig(options.InfraConfigPath)
	if err != nil {
		return errors.Wrap(err, "could not read Azure credentials from infrastructure configuration file")
	}

	// Create Azure clients
	azureClients, err := azure.NewClients(credentials)
	if err != nil {
		return errors.Wrap(err, "could not create Azure clients")
	}

	return remedycontroller.Add(mgr, remedycontroller.AddArgs{
		Actuator: NewActuator(utilsazure.NewPublicIPAddressUtils(azureClients, credentials.ResourceGroup, utilsazure.ReadRequestsCounter, utilsazure.WriteRequestsCounter),
			options.Config, utils.TimestamperFunc(metav1.Now), log.Log.WithName(ActuatorName), CleanedIPsCounter),
		ControllerName:    ControllerName,
		FinalizerName:     FinalizerName,
		ControllerOptions: options.Controller,
		Type:              &azurev1alpha1.PublicIPAddress{},
		Predicates: []predicate.Predicate{
			predicate.GenerationChangedPredicate{},
		},
	})
}

// AddToManager adds a controller with the default AddOptions to the given manager.
func AddToManager(mgr manager.Manager) error {
	return AddToManagerWithOptions(mgr, DefaultAddOptions)
}

func init() {
	// Register metrics with the global Prometheus registry
	metrics.Registry.MustRegister(CleanedIPsCounter)
}
