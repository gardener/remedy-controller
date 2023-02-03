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
	controllercmd "github.com/gardener/gardener/extensions/pkg/controller/cmd"
	"github.com/spf13/pflag"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

const (
	// NamespaceFlag is the name of the command line flag to specify the namespace to watch objects in.
	NamespaceFlag = "namespace"
)

// ManagerOptions are command line options that can be set for manager.Options.
type ManagerOptions struct {
	controllercmd.ManagerOptions
	// Namespace is the namespace to watch objects in.
	// If not specified, defaults to all namespaces.
	Namespace string
	// MetricsBindAddress is the TCP address that the controller should bind to for serving prometheus metrics.
	// It can be set to "0" to disable the metrics serving.
	MetricsBindAddress string

	config *ManagerConfig
}

// AddFlags implements Flagger.AddFlags.
func (m *ManagerOptions) AddFlags(fs *pflag.FlagSet) {
	m.ManagerOptions.AddFlags(fs)
	fs.StringVar(&m.Namespace, NamespaceFlag, m.Namespace, "The namespace to watch objects in.")
}

// Complete implements Completer.Complete.
func (m *ManagerOptions) Complete() error {
	if err := m.ManagerOptions.Complete(); err != nil {
		return err
	}
	m.config = &ManagerConfig{
		ManagerConfig:      *m.ManagerOptions.Completed(),
		Namespace:          m.Namespace,
		MetricsBindAddress: m.MetricsBindAddress,
	}
	return nil
}

// Completed returns the completed ManagerConfig. Only call this if `Complete` was successful.
func (m *ManagerOptions) Completed() *ManagerConfig {
	return m.config
}

// ManagerConfig is a completed manager configuration.
type ManagerConfig struct {
	controllercmd.ManagerConfig
	// Namespace is the namespace to watch objects in.
	// If not specified, defaults to all namespaces.
	Namespace string
	// MetricsBindAddress is the TCP address that the controller should bind to for serving prometheus metrics.
	// It can be set to "0" to disable the metrics serving.
	MetricsBindAddress string
}

// Apply sets the values of this ManagerConfig in the given manager.Options.
func (c *ManagerConfig) Apply(opts *manager.Options) {
	c.ManagerConfig.Apply(opts)
	opts.Namespace = c.Namespace
	opts.MetricsBindAddress = c.MetricsBindAddress
}

// Options initializes empty manager.Options, applies the set values and returns it.
func (c *ManagerConfig) Options() manager.Options {
	var opts manager.Options
	c.Apply(&opts)
	return opts
}
