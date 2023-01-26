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
	"sync"

	azureinstall "github.com/gardener/remedy-controller/pkg/apis/azure/install"
	"github.com/gardener/remedy-controller/pkg/cmd"
	azurenode "github.com/gardener/remedy-controller/pkg/controller/azure/node"
	azurepublicipaddress "github.com/gardener/remedy-controller/pkg/controller/azure/publicipaddress"
	azureservice "github.com/gardener/remedy-controller/pkg/controller/azure/service"
	azurevirtualmachine "github.com/gardener/remedy-controller/pkg/controller/azure/virtualmachine"
	"github.com/gardener/remedy-controller/pkg/version"

	"github.com/gardener/gardener/extensions/pkg/controller"
	controllercmd "github.com/gardener/gardener/extensions/pkg/controller/cmd"
	"github.com/gardener/gardener/extensions/pkg/util"
	"github.com/spf13/cobra"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

const (
	// Name is the name of the Azure remedy controller command.
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
			Namespace:          "kube-system",
		}

		targetRestOpts = &controllercmd.RESTOptions{}
		targetMgrOpts  = &cmd.ManagerOptions{
			ManagerOptions:     controllercmd.ManagerOptions{},
			MetricsBindAddress: ":6001",
		}

		// options for the publicipaddress controller
		publicIPAddressCtrlOpts = &controllercmd.ControllerOptions{
			MaxConcurrentReconciles: 5,
		}

		// options for the virtualmachine controller
		virtualMachineCtrlOpts = &controllercmd.ControllerOptions{
			MaxConcurrentReconciles: 5,
		}

		// options for the service controller
		serviceCtrlOpts = &controllercmd.ControllerOptions{
			MaxConcurrentReconciles: 5,
		}

		// options for the node controller
		nodeCtrlOpts = &controllercmd.ControllerOptions{
			MaxConcurrentReconciles: 5,
		}

		configFileOpts           = &cmd.ConfigOptions{}
		controllerSwitches       = cmd.ControllerSwitchOptions()
		targetControllerSwitches = cmd.TargetControllerSwitchOptions()
		reconcilerOpts           = &cmd.ReconcilerOptions{}

		aggOption = controllercmd.NewOptionAggregator(
			restOpts,
			mgrOpts,
			controllercmd.PrefixOption("target-", targetRestOpts),
			controllercmd.PrefixOption("target-", targetMgrOpts),
			controllercmd.PrefixOption("publicipaddress-", publicIPAddressCtrlOpts),
			controllercmd.PrefixOption("virtualmachine-", virtualMachineCtrlOpts),
			controllercmd.PrefixOption("service-", serviceCtrlOpts),
			controllercmd.PrefixOption("node-", nodeCtrlOpts),
			configFileOpts,
			controllerSwitches,
			controllercmd.PrefixOption("target-", targetControllerSwitches),
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
				logErrAndExit(err, "Could not complete options")
			}

			util.ApplyClientConnectionConfigurationToRESTConfig(configFileOpts.Completed().Config.ClientConnection, restOpts.Completed().Config)
			util.ApplyClientConnectionConfigurationToRESTConfig(configFileOpts.Completed().Config.ClientConnection, targetRestOpts.Completed().Config)

			logger.Info("Creating managers")
			mgr, err := manager.New(restOpts.Completed().Config, mgrOpts.Completed().Options())
			if err != nil {
				logErrAndExit(err, "Could not create manager")
			}
			targetMgr, err := manager.New(targetRestOpts.Completed().Config, targetMgrOpts.Completed().Options())
			if err != nil {
				logErrAndExit(err, "Could not create target cluster manager")
			}

			logger.Info("Updating manager schemes")
			scheme := mgr.GetScheme()
			if err := controller.AddToScheme(scheme); err != nil {
				logErrAndExit(err, "Could not update manager scheme")
			}
			if err := azureinstall.AddToScheme(scheme); err != nil {
				logErrAndExit(err, "Could not update manager scheme")
			}
			targetScheme := targetMgr.GetScheme()
			if err := controller.AddToScheme(targetScheme); err != nil {
				logErrAndExit(err, "Could not update target cluster manager scheme")
			}

			publicIPAddressCtrlOpts.Completed().Apply(&azurepublicipaddress.DefaultAddOptions.Controller)
			configFileOpts.Completed().ApplyAzureOrphanedPublicIPRemedy(&azurepublicipaddress.DefaultAddOptions.Config)
			configFileOpts.Completed().ApplyAzureOrphanedPublicIPRemedy(&azureservice.DefaultAddOptions.Config)
			virtualMachineCtrlOpts.Completed().Apply(&azurevirtualmachine.DefaultAddOptions.Controller)
			configFileOpts.Completed().ApplyAzureFailedVMRemedy(&azurevirtualmachine.DefaultAddOptions.Config)
			configFileOpts.Completed().ApplyAzureFailedVMRemedy(&azurenode.DefaultAddOptions.Config)
			serviceCtrlOpts.Completed().Apply(&azureservice.DefaultAddOptions.Controller)
			nodeCtrlOpts.Completed().Apply(&azurenode.DefaultAddOptions.Controller)
			reconcilerOpts.Completed().Apply(&azurepublicipaddress.DefaultAddOptions.InfraConfigPath)
			reconcilerOpts.Completed().Apply(&azurevirtualmachine.DefaultAddOptions.InfraConfigPath)
			azureservice.DefaultAddOptions.Client = mgr.GetClient()
			azureservice.DefaultAddOptions.Namespace = mgrOpts.Completed().Namespace
			azureservice.DefaultAddOptions.Manager = mgr
			azurenode.DefaultAddOptions.Client = mgr.GetClient()
			azurenode.DefaultAddOptions.Namespace = mgrOpts.Completed().Namespace
			azurenode.DefaultAddOptions.Manager = mgr

			logger.Info("Adding controllers to managers")
			if err := controllerSwitches.Completed().AddToManager(mgr); err != nil {
				logErrAndExit(err, "Could not add controllers to manager")
			}
			if err := targetControllerSwitches.Completed().AddToManager(targetMgr); err != nil {
				logErrAndExit(err, "Could not add controllers to target cluster manager")
			}

			logger.Info("Starting managers")
			var wg sync.WaitGroup
			wg.Add(2)
			go func() {
				defer wg.Done()
				if err := mgr.Start(ctx); err != nil {
					logErrAndExit(err, "Error starting manager")
				}
			}()
			go func() {
				defer wg.Done()
				if err := targetMgr.Start(ctx); err != nil {
					logErrAndExit(err, "Error starting target cluster manager")
				}
			}()
			wg.Wait()
		},
	}

	aggOption.AddFlags(cmd.Flags())

	return cmd
}

func logErrAndExit(err error, msg string) {
	log.Log.Error(err, msg)
	os.Exit(1)
}
