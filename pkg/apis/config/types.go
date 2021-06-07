// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package config

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	componentbaseconfig "k8s.io/component-base/config"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ControllerConfiguration defines the configuration for the remedy controller.
type ControllerConfiguration struct {
	metav1.TypeMeta

	// ClientConnection specifies the kubeconfig file and client connection
	// settings for the proxy server to use when communicating with the apiserver.
	ClientConnection *componentbaseconfig.ClientConnectionConfiguration

	// Azure specifies the configuration for all Azure remedies.
	Azure *AzureConfiguration
}

// AzureConfiguration defines the configuration for the Azure remedy controller.
type AzureConfiguration struct {
	OrphanedPublicIPRemedy *AzureOrphanedPublicIPRemedyConfiguration
	FailedVMRemedy         *AzureFailedVMRemedyConfiguration
}

// AzureOrphanedPublicIPRemedyConfiguration defines the configuration for the Azure orphaned public IP remedy.
type AzureOrphanedPublicIPRemedyConfiguration struct {
	// RequeueInterval specifies the time after which reconciliation requests will be
	// requeued in case of an error or a transient state. Applies to both creation/update and deletion.
	RequeueInterval metav1.Duration
	// SyncPeriod determines the minimum frequency at which PublicIPAddress resources will be reconciled.
	// Only applies to creation/update.
	SyncPeriod metav1.Duration
	// DeletionGracePeriod specifies the period after which a public ip address will be
	// deleted by the controller if it still exists.
	DeletionGracePeriod metav1.Duration
	// MaxGetAttempts specifies the max attempts to get an Azure public ip address.
	MaxGetAttempts int
	// MaxCleanAttempts specifies the max attempts to clean an Azure public ip address.
	MaxCleanAttempts int
}

// AzureFailedVMRemedyConfiguration defines the configuration for the Azure failed VM remedy.
type AzureFailedVMRemedyConfiguration struct {
	// RequeueInterval specifies the time after which reconciliation requests will be
	// requeued in case of an error or a transient state. Applies to both creation/update and deletion.
	RequeueInterval metav1.Duration
	// SyncPeriod determines the minimum frequency at which VirtualMachine resources will be reconciled.
	// Only applies to creation/update.
	SyncPeriod metav1.Duration
	// MaxGetAttempts specifies the max attempts to get an Azure VM.
	MaxGetAttempts int
	// MaxReapplyAttempts specifies the max attempts to reapply an Azure VM.
	MaxReapplyAttempts int
}
