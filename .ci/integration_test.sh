#!/usr/bin/env bash

set -e

repo_dir="$(readlink -f "$(dirname "${0}")/..")"

# HACK: will fail at installation of python, which is already provided - we only need helm in this image
make -C "${repo_dir}" install-requirements || true

pip3 install -r "${repo_dir}/test/requirements.txt"

python3 "${repo_dir}/.ci/integration_test.py" \
    --kubeconfig-name remedy-test-cluster --credentials-config-name integration_test
