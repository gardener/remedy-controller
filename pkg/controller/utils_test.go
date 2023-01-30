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

package controller_test

import (
	. "github.com/gardener/remedy-controller/pkg/controller"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	label = "test"
)

var _ = Describe("Utils", func() {
	Describe("ClusterObjectLabeler", func() {
		var (
			labeler = NewClusterObjectLabeler()
			node    = &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: name,
				},
			}
		)

		DescribeTable("#GetLabelValue",
			func(obj client.Object, labelValue string) {
				Expect(labeler.GetLabelValue(obj)).To(Equal(labelValue))
			},
			Entry("node with name", node, name),
			Entry("node without name", &corev1.Node{}, ""),
		)

		DescribeTable("#GetNamespacedName",
			func(labelValue string, namespacedName types.NamespacedName) {
				Expect(labeler.GetNamespacedName(labelValue)).To(Equal(namespacedName))
			},
			Entry("non-empty label value", name, types.NamespacedName{Name: name}),
			Entry("empty label value", "", types.NamespacedName{}),
		)
	})

	Describe("NamespacedObjectLabeler", func() {
		var (
			labeler = NewNamespacedObjectLabeler(".")
			svc     = &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
				},
			}
		)

		DescribeTable("#GetLabelValue",
			func(obj client.Object, labelValue string) {
				Expect(labeler.GetLabelValue(obj)).To(Equal(labelValue))
			},
			Entry("svc with name", svc, namespace+"."+name),
			Entry("svc without name", &corev1.Service{}, ""),
		)

		DescribeTable("#GetNamespacedName",
			func(labelValue string, namespacedName types.NamespacedName) {
				Expect(labeler.GetNamespacedName(labelValue)).To(Equal(namespacedName))
			},
			Entry("non-empty label value", namespace+"."+name, types.NamespacedName{Namespace: namespace, Name: name}),
			Entry("empty label value", "", types.NamespacedName{}),
		)
	})

	Describe("LabelMapper", func() {
		var (
			mapper = NewLabelMapper(NewNamespacedObjectLabeler("."), label)
			pod    = &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						label: namespace + "." + name,
					},
				},
			}
		)

		DescribeTable("#Map",
			func(obj client.Object, objectKey client.ObjectKey) {
				Expect(mapper.Map(obj)).To(Equal(objectKey))
			},
			Entry("labeled object", pod, client.ObjectKey{Namespace: namespace, Name: name}),
			Entry("unlabeled object", &corev1.Pod{}, client.ObjectKey{}),
		)

		DescribeTable("MapFuncFromMapper",
			func(obj client.Object, requests []reconcile.Request) {
				Expect(MapFuncFromMapper(mapper)(obj)).To(Equal(requests))
			},
			Entry("labeled object", pod, []reconcile.Request{
				{NamespacedName: types.NamespacedName{Namespace: namespace, Name: name}},
			}),
			Entry("unlabeled object", &corev1.Pod{}, nil),
		)
	})
})
