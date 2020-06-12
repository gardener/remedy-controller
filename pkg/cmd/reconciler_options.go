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
	"github.com/spf13/pflag"
)

// ReconcilerOptions are command line options that can be set for controller.Options.
type ReconcilerOptions struct {
	// InfraConfigPath is the path to the infrastructure configuration file.
	InfraConfigPath string

	config *ReconcilerConfig
}

// AddFlags implements Flagger.AddFlags.
func (c *ReconcilerOptions) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&c.InfraConfigPath, "infrastructure-config", "", "path to the infrastructure configuration file")
}

// Complete implements Completer.Complete.
func (c *ReconcilerOptions) Complete() error {
	c.config = &ReconcilerConfig{c.InfraConfigPath}
	return nil
}

// Completed returns the completed ReconcilerConfig. Only call this if `Complete` was successful.
func (c *ReconcilerOptions) Completed() *ReconcilerConfig {
	return c.config
}

// ReconcilerConfig is a completed controller configuration.
type ReconcilerConfig struct {
	// InfraConfigPath is the path to the infrastructure configuration file.
	InfraConfigPath string
}

// Apply sets the values of this ReconcilerConfig in the given controller.Options.
func (c *ReconcilerConfig) Apply(path *string) {
	*path = c.InfraConfigPath
}
