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

const (
	// IgnoreAnnotation is an annotation that can be used to specify that a particular service should be ignored.
	IgnoreAnnotation = "azure.remedy.gardener.cloud/ignore"
	// DoNotCleanAnnotation is an annotation that can be used to specify that a particular PublicIPAddress
	// should be not be cleaned when deleted.
	DoNotCleanAnnotation = "azure.remedy.gardener.cloud/do-not-clean"

	// ServiceLabel is the label to put on a PublicIPAddress object that identifies its service.
	ServiceLabel = "azure.remedy.gardener.cloud/service"
	// NodeLabel is the label to put on a VirtualMachine object that identifies its node.
	NodeLabel = "azure.remedy.gardener.cloud/node"
)
