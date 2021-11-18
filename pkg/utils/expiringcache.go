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

package utils

import "time"

// ExpiringCache is a map whose entries expire after a per-entry timeout.
type ExpiringCache interface {
	// Get looks up an entry in the cache.
	Get(key interface{}) (value interface{}, ok bool)
	// Set sets a key/value/ttl entry in the cache, overwriting any previous entry with the same key.
	Set(key interface{}, value interface{}, ttl time.Duration)
	// Delete deletes an entry from the cache.
	Delete(key interface{})
}
