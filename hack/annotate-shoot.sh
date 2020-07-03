#!/bin/bash
#
# Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#      http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -eu

project_name="$1"
shoot_name="${2:-}"

if [[ -z ${KUBECONFIG:-} ]]; then
    echo "KUBECONFIG env var must be set"
    exit 1
fi

if [[ -z $shoot_name ]]; then
    shoots="$(kubectl get shoots -n "garden-$project_name" --no-headers -o custom-columns=":metadata.name")"

    for s in $shoots; do
        provider="$(kubectl get shoots -n "garden-$project_name" "$s" -ojsonpath={.spec.provider.type})"

        if [[ $provider = "azure" ]]; then
            echo "Patching shoot $s in project $project_name"
            kubectl patch shoot -n "garden-$project_name" "$s" -p \
                '{"metadata":{"annotations":{"azure.provider.extensions.gardener.cloud/enable-remedy-controller":"true"}}}'
        fi
    done
else
    provider="$(kubectl get shoots -n "garden-$project_name" "$shoot_name" -ojsonpath={.spec.provider.type})"

    if [[ $provider = "azure" ]]; then
        echo "Patching shoot $shoot_name in project $project_name"
        kubectl patch shoot -n "garden-$project_name" "$shoot_name" -p \
            '{"metadata":{"annotations":{"azure.provider.extensions.gardener.cloud/enable-remedy-controller":"true"}}}'
    fi
fi

