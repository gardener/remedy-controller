// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package node_test

import (
	azurenode "github.com/gardener/remedy-controller/pkg/controller/azure/node"
	mockutils "github.com/gardener/remedy-controller/pkg/mock/remedy-controller/utils"

	"github.com/go-logr/logr"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

var _ = Describe("Predicate", func() {
	const (
		nodeName = "test-node"
	)

	var (
		ctrl *gomock.Controller

		nodeCache *mockutils.MockExpiringCache

		logger logr.Logger
		p      predicate.Predicate

		node       *corev1.Node
		projection *azurenode.Projection
		now        metav1.Time
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())

		nodeCache = mockutils.NewMockExpiringCache(ctrl)

		logger = log.Log.WithName("test")
		p = azurenode.NewPredicate(nodeCache, logger)

		node = &corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name: nodeName,
			},
			Status: corev1.NodeStatus{
				Conditions: []corev1.NodeCondition{
					{
						Type:   corev1.NodeReady,
						Status: corev1.ConditionTrue,
					},
				},
			},
		}
		projection = &azurenode.Projection{}
		now = metav1.Now()
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#Create", func() {
		It("should return false with an empty event", func() {
			Expect(p.Create(event.CreateEvent{})).To(Equal(false))
		})

		It("should return false with an object that is not a node", func() {
			Expect(p.Create(event.CreateEvent{Object: &corev1.Service{}})).To(Equal(false))
		})

		It("should return true with an object that is a node (and add it to the node cache)", func() {
			nodeCache.EXPECT().Set(nodeName, projection, azurenode.CacheTTL)

			Expect(p.Create(event.CreateEvent{Object: node})).To(Equal(true))
		})
	})

	Describe("#Update", func() {
		It("should return false with an empty event", func() {
			Expect(p.Update(event.UpdateEvent{})).To(Equal(false))
		})

		It("should return false with an old object that is not a node", func() {
			Expect(p.Update(event.UpdateEvent{ObjectOld: &corev1.Service{}, ObjectNew: node})).To(Equal(false))
		})

		It("should return false with a new object that is not a node", func() {
			Expect(p.Update(event.UpdateEvent{ObjectOld: node, ObjectNew: &corev1.Service{}})).To(Equal(false))
		})

		It("should return true if the new node is missing from the node cache (and add it to the node cache)", func() {
			nodeCache.EXPECT().Get(nodeName).Return(nil, false)
			nodeCache.EXPECT().Set(nodeName, projection, azurenode.CacheTTL)

			Expect(p.Update(event.UpdateEvent{ObjectNew: node, ObjectOld: node})).To(Equal(true))
		})

		It("should return true if the deletion timestamp of the new node is different from that of the old node", func() {
			newNode := node.DeepCopy()
			newNode.DeletionTimestamp = &now
			newProjection := &azurenode.Projection{DeletionTimestamp: &now}
			nodeCache.EXPECT().Get(nodeName).Return(newProjection, true)
			nodeCache.EXPECT().Set(nodeName, newProjection, azurenode.CacheTTL)

			Expect(p.Update(event.UpdateEvent{ObjectNew: newNode, ObjectOld: node})).To(Equal(true))
		})

		It("should return true if the deletion timestamp of the new node is different from that of the cached node", func() {
			newNode := node.DeepCopy()
			newNode.DeletionTimestamp = &now
			newProjection := &azurenode.Projection{DeletionTimestamp: &now}
			nodeCache.EXPECT().Get(nodeName).Return(projection, true)
			nodeCache.EXPECT().Set(nodeName, newProjection, azurenode.CacheTTL)

			Expect(p.Update(event.UpdateEvent{ObjectNew: newNode, ObjectOld: newNode})).To(Equal(true))
		})

		It("should return true if the 'not ready or unreachable' status of the new node is different from that of the old node", func() {
			newNode := node.DeepCopy()
			newNode.Status.Conditions[0].Status = corev1.ConditionFalse
			newProjection := &azurenode.Projection{NotReadyOrUnreachable: true}
			nodeCache.EXPECT().Get(nodeName).Return(newProjection, true)
			nodeCache.EXPECT().Set(nodeName, newProjection, azurenode.CacheTTL)

			Expect(p.Update(event.UpdateEvent{ObjectNew: newNode, ObjectOld: node})).To(Equal(true))
		})

		It("should return true if the 'not ready or unreachable' status of the new node is different from that of the cached node", func() {
			newNode := node.DeepCopy()
			newNode.Status.Conditions[0].Status = corev1.ConditionFalse
			newProjection := &azurenode.Projection{NotReadyOrUnreachable: true}
			nodeCache.EXPECT().Get(nodeName).Return(projection, true)
			nodeCache.EXPECT().Set(nodeName, newProjection, azurenode.CacheTTL)

			Expect(p.Update(event.UpdateEvent{ObjectNew: newNode, ObjectOld: newNode})).To(Equal(true))
		})

		It("should return false if the new node is not different from the old or the cached node", func() {
			nodeCache.EXPECT().Get(nodeName).Return(projection, true)

			Expect(p.Update(event.UpdateEvent{ObjectNew: node, ObjectOld: node})).To(Equal(false))
		})
	})

	Describe("#Delete", func() {
		It("should return false with an empty event", func() {
			Expect(p.Delete(event.DeleteEvent{})).To(Equal(false))
		})

		It("should return false with an object that is not a node", func() {
			Expect(p.Delete(event.DeleteEvent{Object: &corev1.Service{}})).To(Equal(false))
		})

		It("should return true with an object that is a node (and delete it from the node cache)", func() {
			nodeCache.EXPECT().Delete(nodeName)

			Expect(p.Delete(event.DeleteEvent{Object: node})).To(Equal(true))
		})
	})
})
