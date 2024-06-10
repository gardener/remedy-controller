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

package node

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	azurev1alpha1 "github.com/gardener/remedy-controller/pkg/apis/azure/v1alpha1"
	"github.com/gardener/remedy-controller/pkg/controller"
	"github.com/gardener/remedy-controller/pkg/controller/azure"
)

const (
	// TaintKeyUnreachable is the taint key to determine whether a node is unreachable.
	TaintKeyUnreachable = "node.kubernetes.io/unreachable"
	// HostnameLabel is a label to determine the hostname of a node.
	HostnameLabel = "kubernetes.io/hostname"
)

type actuator struct {
	client     client.Client
	namespace  string
	syncPeriod time.Duration
	logger     logr.Logger
}

// NewActuator creates a new Actuator.
func NewActuator(client client.Client, namespace string, syncPeriod time.Duration, logger logr.Logger) controller.Actuator {
	logger.Info("Creating actuator", "namespace", namespace, "syncPeriod", syncPeriod)
	return &actuator{
		client:     client,
		namespace:  namespace,
		syncPeriod: syncPeriod,
		logger:     logger,
	}
}

// CreateOrUpdate reconciles object creation or update.
func (a *actuator) CreateOrUpdate(ctx context.Context, obj client.Object) (requeueAfter time.Duration, err error) {
	// Cast object to Node
	var node *corev1.Node
	var ok bool
	if node, ok = obj.(*corev1.Node); !ok {
		return 0, errors.New("reconciled object is not a node")
	}

	// Initialize labels
	vmLabels := map[string]string{
		azure.NodeLabel: ObjectLabeler.GetLabelValue(node),
	}

	// Get node properties
	hostname := node.Labels[HostnameLabel]
	providerID := node.Spec.ProviderID
	notReadyOrUnreachable := isNodeNotReadyOrUnreachable(node)

	// Create or update the VirtualMachine object for the node
	vm := &azurev1alpha1.VirtualMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      node.Name,
			Namespace: a.namespace,
		},
	}
	a.logger.Info("Creating or updating virtualmachine", "name", vm.Name, "namespace", vm.Namespace)
	if err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		_, err := controllerutil.CreateOrUpdate(ctx, a.client, vm, func() error {
			vm.Labels = vmLabels
			vm.Spec.Hostname = hostname
			vm.Spec.ProviderID = providerID
			vm.Spec.NotReadyOrUnreachable = notReadyOrUnreachable
			return nil
		})
		return err
	}); err != nil {
		return 0, errors.Wrap(err, "could not create or update virtualmachine")
	}

	return a.syncPeriod, nil
}

// Delete reconciles object deletion.
func (a *actuator) Delete(ctx context.Context, obj client.Object) (requeueAfter time.Duration, err error) {
	// Cast object to Node
	var node *corev1.Node
	var ok bool
	if node, ok = obj.(*corev1.Node); !ok {
		return 0, errors.New("reconciled object is not a Node")
	}

	// Delete the VirtualMachine object for the node
	vm := &azurev1alpha1.VirtualMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      node.Name,
			Namespace: a.namespace,
		},
	}
	a.logger.Info("Deleting virtualmachine", "name", vm.Name, "namespace", vm.Namespace)
	if err := client.IgnoreNotFound(a.client.Delete(ctx, vm)); err != nil {
		return 0, errors.Wrap(err, "could not delete virtualmachine")
	}

	return 0, nil
}

// ShouldFinalize returns true if the object should be finalized.
func (a *actuator) ShouldFinalize(_ context.Context, _ client.Object) (bool, error) {
	return true, nil
}

func isNodeNotReadyOrUnreachable(node *corev1.Node) bool {
	return !isNodeReady(node) || isNodeUnreachable(node)
}

func isNodeReady(node *corev1.Node) bool {
	for _, condition := range node.Status.Conditions {
		if condition.Type == corev1.NodeReady {
			return condition.Status == corev1.ConditionTrue
		}
	}
	return false
}

func isNodeUnreachable(node *corev1.Node) bool {
	for _, taint := range node.Spec.Taints {
		if taint.Key == TaintKeyUnreachable {
			return true
		}
	}
	return false
}
