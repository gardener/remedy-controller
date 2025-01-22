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

package azure

import (
	"context"
	"io"
	"os"
	"path/filepath"

	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2019-07-01/compute"
	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2018-11-01/network"
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/adal"
	"github.com/Azure/go-autorest/autorest/azure"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
)

// Credentials contains credentials and other parameters needed to work with Azure objects.
type Credentials struct {
	ClientID       string `yaml:"aadClientId"`
	ClientSecret   string `yaml:"aadClientSecret"`
	TenantID       string `yaml:"tenantId"`
	SubscriptionID string `yaml:"subscriptionId"`
	ResourceGroup  string `yaml:"resourceGroup"`
}

// Future contains the method WaitForCompletionRef.
type Future interface {
	// WaitForCompletionRef will return when one of the following conditions is met ...
	WaitForCompletionRef(context.Context, autorest.Client) error
}

// PublicIPAddressesClient contains the methods of network.PublicIPAddressesClient.
type PublicIPAddressesClient interface {
	// Get gets the specified public IP address in a specified resource group.
	Get(context.Context, string, string, string) (network.PublicIPAddress, error)
	// List gets all public IP addresses in a resource group.
	List(context.Context, string) (network.PublicIPAddressListResultPage, error)
	// Delete deletes the specified public IP address.
	Delete(context.Context, string, string) (Future, error)
	// Client returns the autorest.Client
	Client() autorest.Client
}

// LoadBalancersClient contains the methods of network.LoadBalancersClient.
type LoadBalancersClient interface {
	// Get gets the specified load balancer.
	Get(context.Context, string, string, string) (network.LoadBalancer, error)
	// CreateOrUpdate creates or updates a load balancer.
	CreateOrUpdate(context.Context, string, string, network.LoadBalancer) (Future, error)
	// Client returns the autorest.Client
	Client() autorest.Client
}

// VirtualMachinesClient contains the methods of compute.VirtualMachinesClient.
type VirtualMachinesClient interface {
	// Get gets the specified virtual machine.
	Get(context.Context, string, string, compute.InstanceViewTypes) (compute.VirtualMachine, error)
	// Reapply reapplies the virtual machine's state.
	Reapply(context.Context, string, string) (Future, error)
	// Client returns the autorest.Client
	Client() autorest.Client
}

// PublicIPAddressesClientImpl is an implementation of PublicIPAddressesClient based on network.PublicIPAddressesClient.
type PublicIPAddressesClientImpl struct {
	network.PublicIPAddressesClient
}

// Delete implements PublicIPAddressesClient.
func (c PublicIPAddressesClientImpl) Delete(ctx context.Context, resourceGroupName string, publicIPAddressName string) (Future, error) {
	f, err := c.PublicIPAddressesClient.Delete(ctx, resourceGroupName, publicIPAddressName)
	return &f, err
}

// Client implements PublicIPAddressesClient.
func (c PublicIPAddressesClientImpl) Client() autorest.Client {
	return c.PublicIPAddressesClient.Client
}

// LoadBalancersClientImpl is an implementation of LoadBalancersClient based on network.LoadBalancersClient.
type LoadBalancersClientImpl struct {
	network.LoadBalancersClient
}

// CreateOrUpdate implements LoadBalancersClient.
func (c LoadBalancersClientImpl) CreateOrUpdate(ctx context.Context, resourceGroupName string, loadBalancerName string, loadBalancer network.LoadBalancer) (Future, error) {
	f, err := c.LoadBalancersClient.CreateOrUpdate(ctx, resourceGroupName, loadBalancerName, loadBalancer)
	return &f, err
}

// Client implements LoadBalancersClient.
func (c LoadBalancersClientImpl) Client() autorest.Client {
	return c.LoadBalancersClient.Client
}

// VirtualMachinesClientImpl is an implementation of VirtualMachinesClient based on compute.VirtualMachinesClient.
type VirtualMachinesClientImpl struct {
	compute.VirtualMachinesClient
}

// Reapply implements VirtualMachinesClient.
func (c VirtualMachinesClientImpl) Reapply(ctx context.Context, resourceGroupName string, vmName string) (Future, error) {
	f, err := c.VirtualMachinesClient.Reapply(ctx, resourceGroupName, vmName)
	return &f, err
}

// Client implements VirtualMachinesClient.
func (c VirtualMachinesClientImpl) Client() autorest.Client {
	return c.VirtualMachinesClient.Client
}

// Clients contains all needed Azure clients.
type Clients struct {
	PublicIPAddressesClient PublicIPAddressesClient
	LoadBalancersClient     LoadBalancersClient
	VirtualMachinesClient   VirtualMachinesClient
}

// ReadConfig creates new Azure credentials by reading the configuration file at the given path.
func ReadConfig(path string) (*Credentials, error) {
	// Open the configuration file
	input, err := os.Open(filepath.Clean(path))
	if err != nil {
		return nil, errors.Wrap(err, "could not open configuration file")
	}
	defer input.Close() // nolint:errcheck

	// Decode Azure credentials from JSON
	decoder := yaml.NewDecoder(io.Reader(input))
	credentials := &Credentials{}
	if err := decoder.Decode(credentials); err != nil {
		return nil, errors.Wrap(err, "could not decode Azure credentials from JSON")
	}
	return credentials, nil
}

// NewClients creates a new Clients instance using the given credentials.
func NewClients(credentials *Credentials) (*Clients, error) {
	// Create OAuth config
	oauthConfig, err := adal.NewOAuthConfig(azure.PublicCloud.ActiveDirectoryEndpoint, credentials.TenantID)
	if err != nil {
		return nil, errors.Wrap(err, "could not create OAuth config")
	}

	// Create service principal token
	servicePrincipalToken, err := adal.NewServicePrincipalToken(*oauthConfig, credentials.ClientID, credentials.ClientSecret, azure.PublicCloud.ResourceManagerEndpoint)
	if err != nil {
		return nil, errors.Wrap(err, "could not create service principal token")
	}
	authorizer := autorest.NewBearerAuthorizer(servicePrincipalToken)

	// Create clients
	ipAddressesClient := network.NewPublicIPAddressesClient(credentials.SubscriptionID)
	ipAddressesClient.Authorizer = authorizer
	loadBalancersClient := network.NewLoadBalancersClient(credentials.SubscriptionID)
	loadBalancersClient.Authorizer = authorizer
	vmClient := compute.NewVirtualMachinesClient(credentials.SubscriptionID)
	vmClient.Authorizer = authorizer

	return &Clients{
		PublicIPAddressesClient: PublicIPAddressesClientImpl{PublicIPAddressesClient: ipAddressesClient},
		LoadBalancersClient:     LoadBalancersClientImpl{LoadBalancersClient: loadBalancersClient},
		VirtualMachinesClient:   VirtualMachinesClientImpl{VirtualMachinesClient: vmClient},
	}, nil
}
