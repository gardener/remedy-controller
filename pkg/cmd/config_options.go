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

package cmd

import (
	"fmt"

	"github.wdf.sap.corp/kubernetes/remedy-controller/pkg/apis/config"
	confighelper "github.wdf.sap.corp/kubernetes/remedy-controller/pkg/apis/config/helper"

	"github.com/spf13/pflag"
)

// ConfigOptions are command line options that can be set for config.ControllerConfiguration.
type ConfigOptions struct {
	// ConfigFilePath is the path to a config file.
	ConfigFilePath string

	config *Config
}

// Config is a completed controller configuration.
type Config struct {
	// Config is the controller configuration.
	Config *config.ControllerConfiguration
}

func (c *ConfigOptions) buildConfig() (*config.ControllerConfiguration, error) {
	if len(c.ConfigFilePath) == 0 {
		return nil, fmt.Errorf("config file path not set")
	}
	return confighelper.LoadFromFile(c.ConfigFilePath)
}

// Complete implements RESTCompleter.Complete.
func (c *ConfigOptions) Complete() error {
	config, err := c.buildConfig()
	if err != nil {
		return err
	}

	c.config = &Config{config}
	return nil
}

// Completed returns the completed Config. Only call this if `Complete` was successful.
func (c *ConfigOptions) Completed() *Config {
	return c.config
}

// AddFlags implements Flagger.AddFlags.
func (c *ConfigOptions) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&c.ConfigFilePath, "config-file", "", "path to the controller manager configuration file")
}

// Apply sets the values of this Config in the given config.ControllerConfiguration.
func (c *Config) Apply(cfg *config.ControllerConfiguration) {
	*cfg = *c.Config
}

// Options initializes empty config.ControllerConfiguration, applies the set values and returns it.
func (c *Config) Options() config.ControllerConfiguration {
	var cfg config.ControllerConfiguration
	c.Apply(&cfg)
	return cfg
}

// ApplyAzureOrphanedPublicIPRemedy sets the given Azure orphaned public IP remedy configuration to that of this Config.
func (c *Config) ApplyAzureOrphanedPublicIPRemedy(cfg *config.AzureOrphanedPublicIPRemedyConfiguration) {
	if c.Config.Azure != nil && c.Config.Azure.OrphanedPublicIPRemedy != nil {
		*cfg = *c.Config.Azure.OrphanedPublicIPRemedy
	}
}

// ApplyAzureFailedVMRemedy sets the given Azure failed VM remedy configuration to that of this Config.
func (c *Config) ApplyAzureFailedVMRemedy(cfg *config.AzureFailedVMRemedyConfiguration) {
	if c.Config.Azure != nil && c.Config.Azure.FailedVMRemedy != nil {
		*cfg = *c.Config.Azure.FailedVMRemedy
	}
}
