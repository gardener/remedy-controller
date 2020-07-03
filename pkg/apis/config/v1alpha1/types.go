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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	componentbaseconfigv1alpha1 "k8s.io/component-base/config/v1alpha1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ControllerConfiguration defines the configuration for the GCP provider.
type ControllerConfiguration struct {
	metav1.TypeMeta `json:",inline"`

	// ClientConnection specifies the kubeconfig file and client connection
	// settings for the proxy server to use when communicating with the apiserver.
	// +optional
	ClientConnection *componentbaseconfigv1alpha1.ClientConnectionConfiguration `json:"clientConnection,omitempty"`

	// Azure specifies the configuration for all Azure remedies.
	// +optional
	Azure *AzureConfiguration `json:"azure,omitempty"`
}

// AzureConfiguration defines the configuration for the Azure remedy controller.
type AzureConfiguration struct {
	// +optional
	OrphanedPublicIPRemedy *AzureOrphanedPublicIPRemedyConfiguration `json:"orphanedPublicIPRemedy,omitempty"`
	// +optional
	FailedVMRemedy *AzureFailedVMRemedyConfiguration `json:"failedVMRemedy,omitempty"`
}

// AzureOrphanedPublicIPRemedyConfiguration defines the configuration for the Azure orphaned public IP remedy.
type AzureOrphanedPublicIPRemedyConfiguration struct {
	// RequeueInterval specifies the time after which reconciliation requests will be
	// requeued. Applies to both creation/update and deletion.
	// +optional
	RequeueInterval metav1.Duration `json:"requeueInterval,omitempty"`
	// DeletionGracePeriod specifies the period after which a public ip address will be
	// deleted by the controller if it still exists.
	// +optional
	DeletionGracePeriod metav1.Duration `json:"deletionGracePeriod,omitempty"`
	// MaxGetAttempts specifies the max attempts to get an Azure public ip address.
	// +optional
	MaxGetAttempts int `json:"maxGetAttempts,omitempty"`
	// MaxCleanAttempts specifies the max attempts to clean an Azure public ip address.
	// +optional
	MaxCleanAttempts int `json:"maxCleanAttempts,omitempty"`
	// BlacklistedServiceLabels spcifies the labels of services that will be ignored.
	// +optional
	BlacklistedServiceLabels []map[string]string `json:"blacklistedServiceLabels,omitempty"`
}

// AzureFailedVMRemedyConfiguration defines the configuration for the Azure failed VM remedy.
type AzureFailedVMRemedyConfiguration struct {
	// RequeueInterval specifies the time after which reconciliation requests will be
	// requeued. Applies to both creation/update and deletion.
	// +optional
	RequeueInterval metav1.Duration `json:"requeueInterval,omitempty"`
	// MaxGetAttempts specifies the max attempts to get an Azure VM.
	// +optional
	MaxGetAttempts int `json:"maxGetAttempts,omitempty"`
	// MaxReapplyAttempts specifies the max attempts to reapply an Azure VM.
	// +optional
	MaxReapplyAttempts int `json:"maxReapplyAttempts,omitempty"`
}
