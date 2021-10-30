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

package controller

import (
	"strings"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// ObjectLabeler provides methods for creating a label value that uniquely identifies an object,
// as well as creating a namespaced name from such a label value.
type ObjectLabeler interface {
	// GetLabelValue returns a label value that uniquely identifies the given object.
	GetLabelValue(obj client.Object) string
	// GetNamespacedName returns types.NamespacedName from the given label value.
	GetNamespacedName(labelValue string) types.NamespacedName
}

// NewClusterObjectLabeler creates an ObjectLabeler that is appropriate for cluster objects.
// It uses the object name as the label value.
func NewClusterObjectLabeler() ObjectLabeler {
	return &clusterObjectLabeler{}
}

type clusterObjectLabeler struct{}

// GetLabelValue returns a label value that uniquely identifies the given object.
func (l *clusterObjectLabeler) GetLabelValue(obj client.Object) string {
	return obj.GetName()
}

// GetNamespacedName returns types.NamespacedName from the given label value.
func (l *clusterObjectLabeler) GetNamespacedName(labelValue string) types.NamespacedName {
	return types.NamespacedName{Name: labelValue}
}

// NewNamespacedObjectLabeler creates an ObjectLabeler that is appropriate for namespaced objects.
// It uses the object namespace and name separated by the given separator as the label value.
func NewNamespacedObjectLabeler(separator string) ObjectLabeler {
	return &namespacedObjectLabeler{
		separator: separator,
	}
}

type namespacedObjectLabeler struct {
	separator string
}

// GetLabelValue returns a label value that uniquely identifies the given object.
func (l *namespacedObjectLabeler) GetLabelValue(obj client.Object) string {
	if obj.GetName() != "" {
		return obj.GetNamespace() + l.separator + obj.GetName()
	}
	return ""
}

// GetNamespacedName returns types.NamespacedName from the given label value.
func (l *namespacedObjectLabeler) GetNamespacedName(labelValue string) types.NamespacedName {
	if parts := strings.Split(labelValue, l.separator); len(parts) == 2 {
		return types.NamespacedName{Namespace: parts[0], Name: parts[1]}
	}
	return types.NamespacedName{}
}

// Mapper maps an object to the object key of a different object.
type Mapper interface {
	// Map maps the given object to the object key of a different object.
	Map(obj client.Object) client.ObjectKey
}

// MapFuncFromMapper returns handler.MapFunc that uses the given mapper to map the given object
// to the object key of a different object and returns a reconcile.Request for that key if it's not empty.
func MapFuncFromMapper(mapper Mapper) handler.MapFunc {
	return func(obj client.Object) []reconcile.Request {
		key := mapper.Map(obj)
		if key.Name == "" {
			return nil
		}
		return []reconcile.Request{
			{NamespacedName: key},
		}
	}
}

// NewLabelMapper creates a mapper that uses GetNamespacedName of the given ObjectLabeler with the given label.
func NewLabelMapper(objectLabeler ObjectLabeler, label string) Mapper {
	return &labelMapper{
		objectLabeler: objectLabeler,
		label:         label,
	}
}

type labelMapper struct {
	objectLabeler ObjectLabeler
	label         string
}

// Map maps the given object to the object key of a different object.
func (m *labelMapper) Map(obj client.Object) client.ObjectKey {
	return m.objectLabeler.GetNamespacedName(obj.GetLabels()[m.label])
}
