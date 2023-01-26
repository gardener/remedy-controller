//go:build !ignore_autogenerated
// +build !ignore_autogenerated

/*
Copyright (c) 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Code generated by deepcopy-gen. DO NOT EDIT.

package v1alpha1

import (
	runtime "k8s.io/apimachinery/pkg/runtime"
	configv1alpha1 "k8s.io/component-base/config/v1alpha1"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AzureConfiguration) DeepCopyInto(out *AzureConfiguration) {
	*out = *in
	if in.OrphanedPublicIPRemedy != nil {
		in, out := &in.OrphanedPublicIPRemedy, &out.OrphanedPublicIPRemedy
		*out = new(AzureOrphanedPublicIPRemedyConfiguration)
		**out = **in
	}
	if in.FailedVMRemedy != nil {
		in, out := &in.FailedVMRemedy, &out.FailedVMRemedy
		*out = new(AzureFailedVMRemedyConfiguration)
		**out = **in
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AzureConfiguration.
func (in *AzureConfiguration) DeepCopy() *AzureConfiguration {
	if in == nil {
		return nil
	}
	out := new(AzureConfiguration)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AzureFailedVMRemedyConfiguration) DeepCopyInto(out *AzureFailedVMRemedyConfiguration) {
	*out = *in
	out.RequeueInterval = in.RequeueInterval
	out.SyncPeriod = in.SyncPeriod
	out.NodeSyncPeriod = in.NodeSyncPeriod
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AzureFailedVMRemedyConfiguration.
func (in *AzureFailedVMRemedyConfiguration) DeepCopy() *AzureFailedVMRemedyConfiguration {
	if in == nil {
		return nil
	}
	out := new(AzureFailedVMRemedyConfiguration)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AzureOrphanedPublicIPRemedyConfiguration) DeepCopyInto(out *AzureOrphanedPublicIPRemedyConfiguration) {
	*out = *in
	out.RequeueInterval = in.RequeueInterval
	out.SyncPeriod = in.SyncPeriod
	out.ServiceSyncPeriod = in.ServiceSyncPeriod
	out.DeletionGracePeriod = in.DeletionGracePeriod
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AzureOrphanedPublicIPRemedyConfiguration.
func (in *AzureOrphanedPublicIPRemedyConfiguration) DeepCopy() *AzureOrphanedPublicIPRemedyConfiguration {
	if in == nil {
		return nil
	}
	out := new(AzureOrphanedPublicIPRemedyConfiguration)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ControllerConfiguration) DeepCopyInto(out *ControllerConfiguration) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	if in.ClientConnection != nil {
		in, out := &in.ClientConnection, &out.ClientConnection
		*out = new(configv1alpha1.ClientConnectionConfiguration)
		**out = **in
	}
	if in.Azure != nil {
		in, out := &in.Azure, &out.Azure
		*out = new(AzureConfiguration)
		(*in).DeepCopyInto(*out)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ControllerConfiguration.
func (in *ControllerConfiguration) DeepCopy() *ControllerConfiguration {
	if in == nil {
		return nil
	}
	out := new(ControllerConfiguration)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ControllerConfiguration) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}
