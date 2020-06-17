#!/usr/bin/env bash

set -e

repo_dir="$(readlink -f "$(dirname "${0}")/..")"

pip3 install -r "${repo_dir}/test/requirements.txt"

python3 "${repo_dir}/.ci/integration_test.py" \
    --kubeconfig-name remedy-test-cluster --credentials-config-name integration_test