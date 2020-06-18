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

	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2019-07-01/compute"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"

	"github.wdf.sap.corp/kubernetes/remedy-controller/pkg/client/azure"
)

// VirtualMachineUtils provides utility methods for getting and reapplying Azure VirtualMachine objects.
type VirtualMachineUtils interface {
	// Get returns the VirtualMachine with the given name, or nil if not found.
	Get(ctx context.Context, name string) (*compute.VirtualMachine, error)
	// Reapply reapplies the state of the VirtualMachine with the given name.
	Reapply(ctx context.Context, name string) error
}

// NewVirtualMachineUtils creates a new instance of VirtualMachineUtils.
func NewVirtualMachineUtils(
	azureClients *azure.Clients,
	resourceGroup string,
	readRequestsCounter prometheus.Counter,
	writeRequestsCounter prometheus.Counter,
) VirtualMachineUtils {
	return &virtualMachineUtils{
		azureClients:         azureClients,
		resourceGroup:        resourceGroup,
		readRequestsCounter:  readRequestsCounter,
		writeRequestsCounter: writeRequestsCounter,
	}
}

type virtualMachineUtils struct {
	azureClients         *azure.Clients
	resourceGroup        string
	readRequestsCounter  prometheus.Counter
	writeRequestsCounter prometheus.Counter
}

// Get returns the VirtualMachine with the given name, or nil if not found.
func (p *virtualMachineUtils) Get(ctx context.Context, name string) (*compute.VirtualMachine, error) {
	p.readRequestsCounter.Inc()
	azurePublicIP, err := p.azureClients.VirtualMachinesClient.Get(ctx, p.resourceGroup, name, compute.InstanceView)
	if err != nil {
		if isAzureNotFoundError(err) {
			return nil, nil
		}
		return nil, errors.Wrap(err, "could not get Azure VirtualMachine")
	}
	return &azurePublicIP, nil
}

// Reapply reapplies the state of the VirtualMachine with the given name.
func (p *virtualMachineUtils) Reapply(ctx context.Context, name string) error {
	p.writeRequestsCounter.Inc()
	result, err := p.azureClients.VirtualMachinesClient.Reapply(ctx, p.resourceGroup, name)
	if err != nil {
		return errors.Wrap(err, "could not reapply Azure VirtualMachine")
	}
	p.readRequestsCounter.Inc()
	if err := result.WaitForCompletionRef(ctx, p.azureClients.VirtualMachinesClient.Client()); err != nil {
		return errors.Wrap(err, "could not wait for the Azure VirtualMachine reapply to complete")
	}

	return nil
}
