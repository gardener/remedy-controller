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

package app

import (
	"context"
	"os"

	azureinstall "github.wdf.sap.corp/kubernetes/remedy-controller/pkg/apis/azure/install"
	"github.wdf.sap.corp/kubernetes/remedy-controller/pkg/cmd"
	azurepublicipaddress "github.wdf.sap.corp/kubernetes/remedy-controller/pkg/controller/azure/publicipaddress"
	azureservice "github.wdf.sap.corp/kubernetes/remedy-controller/pkg/controller/azure/service"
	"github.wdf.sap.corp/kubernetes/remedy-controller/pkg/version"

	"github.com/gardener/gardener/extensions/pkg/controller"
	controllercmd "github.com/gardener/gardener/extensions/pkg/controller/cmd"
	"github.com/gardener/gardener/extensions/pkg/util"
	"github.com/spf13/cobra"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

const (
	Name = "remedy-controller-azure"
)

// NewControllerManagerCommand creates a new command for running the Azure remedy controller.
func NewControllerManagerCommand(ctx context.Context) *cobra.Command {
	var (
		restOpts = &controllercmd.RESTOptions{}
		mgrOpts  = &cmd.ManagerOptions{
			ManagerOptions: controllercmd.ManagerOptions{
				LeaderElection:          true,
				LeaderElectionID:        controllercmd.LeaderElectionNameID(Name),
				LeaderElectionNamespace: os.Getenv("LEADER_ELECTION_NAMESPACE"),
			},
			MetricsBindAddress: ":6000",
		}

		// options for the publicipaddress controller
		publicIPAddressCtrlOpts = &controllercmd.ControllerOptions{
			MaxConcurrentReconciles: 5,
		}

		// options for the service controller
		serviceCtrlOpts = &controllercmd.ControllerOptions{
			MaxConcurrentReconciles: 5,
		}

		configFileOpts     = &cmd.ConfigOptions{}
		controllerSwitches = cmd.ControllerSwitchOptions()
		reconcilerOpts     = &cmd.ReconcilerOptions{}

		aggOption = controllercmd.NewOptionAggregator(
			restOpts,
			mgrOpts,
			controllercmd.PrefixOption("publicipaddress-", publicIPAddressCtrlOpts),
			controllercmd.PrefixOption("service-", serviceCtrlOpts),
			configFileOpts,
			controllerSwitches,
			reconcilerOpts,
		)
	)

	cmd := &cobra.Command{
		Use: Name,

		Run: func(cmd *cobra.Command, args []string) {
			logger := log.Log.WithName(Name)
			logger.Info("Initializing", "version", version.Version)

			logger.Info("Completing options")
			if err := aggOption.Complete(); err != nil {
				controllercmd.LogErrAndExit(err, "Could not complete options")
			}

			util.ApplyClientConnectionConfigurationToRESTConfig(configFileOpts.Completed().Config.ClientConnection, restOpts.Completed().Config)

			logger.Info("Creating manager")
			mgr, err := manager.New(restOpts.Completed().Config, mgrOpts.Completed().Options())
			if err != nil {
				controllercmd.LogErrAndExit(err, "Could not create manager")
			}

			logger.Info("Updating manager scheme")
			scheme := mgr.GetScheme()
			if err := controller.AddToScheme(scheme); err != nil {
				controllercmd.LogErrAndExit(err, "Could not update manager scheme")
			}
			if err := azureinstall.AddToScheme(scheme); err != nil {
				controllercmd.LogErrAndExit(err, "Could not update manager scheme")
			}

			publicIPAddressCtrlOpts.Completed().Apply(&azurepublicipaddress.DefaultAddOptions.Controller)
			configFileOpts.Completed().ApplyAzurePublicIPRemedy(&azurepublicipaddress.DefaultAddOptions.Config)
			serviceCtrlOpts.Completed().Apply(&azureservice.DefaultAddOptions.Controller)
			reconcilerOpts.Completed().Apply(&azurepublicipaddress.DefaultAddOptions.InfraConfigPath)

			logger.Info("Adding controllers to manager")
			if err := controllerSwitches.Completed().AddToManager(mgr); err != nil {
				controllercmd.LogErrAndExit(err, "Could not add controllers to manager")
			}

			logger.Info("Starting manager")
			if err := mgr.Start(ctx.Done()); err != nil {
				controllercmd.LogErrAndExit(err, "Error running manager")
			}
		},
	}

	aggOption.AddFlags(cmd.Flags())

	return cmd
}
