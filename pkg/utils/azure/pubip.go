// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"net/http"

	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2018-11-01/network"
	"github.com/Azure/go-autorest/autorest"
	gardenerutils "github.com/gardener/gardener/pkg/utils"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"

	"github.wdf.sap.corp/kubernetes/remedy-controller/pkg/client/azure"
)

// PublicIPAddressUtils provides utility methods for getting and cleaning Azure PublicIPAddress objects.
type PublicIPAddressUtils interface {
	// GetByName returns the PublicIPAddress with the given name, or nil if not found.
	GetByName(ctx context.Context, name string) (*network.PublicIPAddress, error)
	// GetByIP returns the PublicIPAddress with the given IP, or nil if not found.
	GetByIP(ctx context.Context, ip string) (*network.PublicIPAddress, error)
	// GetAll returns all PublicIPAddresses.
	GetAll(ctx context.Context) ([]network.PublicIPAddress, error)
	// RemoveFromLoadBalancer removes all FrontendIPConfigurations, LoadBalancingRules, and Probes
	// using the given PublicIPAddress IDs from the LoadBalancer.
	RemoveFromLoadBalancer(ctx context.Context, publicIPAddressIDs []string) error
	// Delete deletes the PublicIPAddress with the given name.
	Delete(ctx context.Context, name string) error
}

// NewPublicIPAddressUtils creates a new instance of PublicIPAddressUtils.
func NewPublicIPAddressUtils(
	azureClients *azure.Clients,
	resourceGroup string,
	readRequestsCounter prometheus.Counter,
	writeRequestsCounter prometheus.Counter,
) PublicIPAddressUtils {
	return &publicIPAddressUtils{
		azureClients:         azureClients,
		resourceGroup:        resourceGroup,
		readRequestsCounter:  readRequestsCounter,
		writeRequestsCounter: writeRequestsCounter,
	}
}

type publicIPAddressUtils struct {
	azureClients         *azure.Clients
	resourceGroup        string
	readRequestsCounter  prometheus.Counter
	writeRequestsCounter prometheus.Counter
}

// GetByName returns the PublicIPAddress with the given name, or nil if not found.
func (p *publicIPAddressUtils) GetByName(ctx context.Context, name string) (*network.PublicIPAddress, error) {
	p.readRequestsCounter.Inc()
	azurePublicIP, err := p.azureClients.PublicIPAddressesClient.Get(ctx, p.resourceGroup, name, "")
	if err != nil {
		if isAzureNotFoundError(err) {
			return nil, nil
		}
		return nil, errors.Wrap(err, "could not get Azure PublicIPAddress")
	}
	return &azurePublicIP, nil
}

// GetByIP returns the PublicIPAddress with the given IP, or nil if not found.
func (p *publicIPAddressUtils) GetByIP(ctx context.Context, ip string) (*network.PublicIPAddress, error) {
	p.readRequestsCounter.Inc()
	azurePublicIPList, err := p.azureClients.PublicIPAddressesClient.List(ctx, p.resourceGroup)
	if err != nil {
		return nil, errors.Wrap(err, "could not list Azure PublicIPAddresses")
	}
	for azurePublicIPList.NotDone() {
		for _, azurePublicIP := range azurePublicIPList.Values() {
			if azurePublicIP.IPAddress != nil && *azurePublicIP.IPAddress == ip {
				return &azurePublicIP, nil
			}
		}
		p.readRequestsCounter.Inc()
		if err := azurePublicIPList.NextWithContext(ctx); err != nil {
			return nil, errors.Wrap(err, "could not advance to the next page of Azure PublicIPAddresses")
		}
	}

	return nil, nil
}

// GetAll returns all PublicIPAddresses.
func (p *publicIPAddressUtils) GetAll(ctx context.Context) ([]network.PublicIPAddress, error) {
	p.readRequestsCounter.Inc()
	azurePublicIPList, err := p.azureClients.PublicIPAddressesClient.List(ctx, p.resourceGroup)
	if err != nil {
		return nil, errors.Wrap(err, "could not list Azure PublicIPAddresses")
	}
	var azurePublicIPs []network.PublicIPAddress
	for azurePublicIPList.NotDone() {
		azurePublicIPs = append(azurePublicIPs, azurePublicIPList.Values()...)
		p.readRequestsCounter.Inc()
		if err := azurePublicIPList.NextWithContext(ctx); err != nil {
			return nil, errors.Wrap(err, "could not advance to the next page of Azure PublicIPAddresses")
		}
	}
	return azurePublicIPs, nil
}

// RemoveFromLoadBalancer removes all FrontendIPConfigurations, LoadBalancingRules, and Probes
// using the given PublicIPAddress IDs from the LoadBalancer.
func (p *publicIPAddressUtils) RemoveFromLoadBalancer(ctx context.Context, publicIPAddressIDs []string) error {
	// Get the Azure LoadBalancer
	lbName := p.resourceGroup // TODO
	p.readRequestsCounter.Inc()
	lb, err := p.azureClients.LoadBalancersClient.Get(ctx, p.resourceGroup, lbName, "")
	if err != nil {
		return errors.Wrap(err, "could not get Azure LoadBalancer")
	}

	// Update the FrontendIPConfigurations, LoadBalancerRules, and Probes on the Azure LoadBalancer
	fcIDs := updateFrontendIPConfigurations(lb, publicIPAddressIDs)
	ruleIDs := updateLoadBalancingRules(lb, fcIDs)
	updateProbes(lb, ruleIDs)
	p.writeRequestsCounter.Inc()
	result, err := p.azureClients.LoadBalancersClient.CreateOrUpdate(ctx, p.resourceGroup, lbName, lb)
	if err != nil {
		return errors.Wrap(err, "could not update Azure LoadBalancer")
	}
	p.readRequestsCounter.Inc()
	if err := result.WaitForCompletionRef(ctx, p.azureClients.LoadBalancersClient.Client()); err != nil {
		return errors.Wrap(err, "could not wait for the Azure LoadBalancer update to complete")
	}

	return nil
}

// Delete deletes the PublicIPAddress with the given name.
func (p *publicIPAddressUtils) Delete(ctx context.Context, name string) error {
	// Delete the Azure PublicIPAddress
	p.writeRequestsCounter.Inc()
	result, err := p.azureClients.PublicIPAddressesClient.Delete(ctx, p.resourceGroup, name)
	if err != nil {
		if isAzureNotFoundError(err) {
			return nil
		}
		return errors.Wrap(err, "could not delete Azure PublicIPAddress")
	}
	p.readRequestsCounter.Inc()
	if err := result.WaitForCompletionRef(ctx, p.azureClients.PublicIPAddressesClient.Client()); err != nil {
		return errors.Wrap(err, "could not wait for the Azure PublicIPAddress deletion to complete")
	}

	return nil
}

func updateFrontendIPConfigurations(lb network.LoadBalancer, publicIPAddressIDs []string) []string {
	if lb.FrontendIPConfigurations == nil {
		return nil
	}
	var fcIDs []string
	var updated []network.FrontendIPConfiguration
	for _, fc := range *lb.FrontendIPConfigurations {
		if fc.ID != nil && fc.PublicIPAddress != nil && fc.PublicIPAddress.ID != nil && gardenerutils.ValueExists(*fc.PublicIPAddress.ID, publicIPAddressIDs) {
			fcIDs = append(fcIDs, *fc.ID)
		} else {
			updated = append(updated, fc)
		}
	}
	*lb.FrontendIPConfigurations = updated
	return fcIDs
}

func updateLoadBalancingRules(lb network.LoadBalancer, fcIDs []string) []string {
	if lb.LoadBalancingRules == nil {
		return nil
	}
	var ruleIDs []string
	var updated []network.LoadBalancingRule
	for _, rule := range *lb.LoadBalancingRules {
		if rule.ID != nil && rule.FrontendIPConfiguration != nil && rule.FrontendIPConfiguration.ID != nil && gardenerutils.ValueExists(*rule.FrontendIPConfiguration.ID, fcIDs) {
			ruleIDs = append(ruleIDs, *rule.ID)
		} else {
			updated = append(updated, rule)
		}
	}
	*lb.LoadBalancingRules = updated
	return ruleIDs
}

func updateProbes(lb network.LoadBalancer, ruleIDs []string) {
	if lb.Probes == nil {
		return
	}
	var updated []network.Probe
	for _, probe := range *lb.Probes {
		if probe.LoadBalancingRules == nil {
			continue
		}
		for _, probeRule := range *probe.LoadBalancingRules {
			if !(probeRule.ID != nil && gardenerutils.ValueExists(*probeRule.ID, ruleIDs)) {
				updated = append(updated, probe)
			}
		}
	}
	*lb.Probes = updated
}

func isAzureNotFoundError(err error) bool {
	if e, ok := err.(autorest.DetailedError); ok {
		return e.StatusCode == http.StatusNotFound
	}
	return false
}
