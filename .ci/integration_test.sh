#!/usr/bin/env bash

set -e

repo_dir="$(readlink -f "$(dirname "${0}")/..")"

# HACK: will fail at installation of python, which is already provided - we only need helm in this image
make -C "${repo_dir}" install-requirements || true

# need to setup the CRDs in the test-cluster
kubectl --kubeconfig "${TM_KUBECONFIG_PATH}/shoot.config" apply -f "${repo_dir}/example/20-crd-publicipaddress.yaml"
kubectl --kubeconfig "${TM_KUBECONFIG_PATH}/shoot.config" apply -f "${repo_dir}/example/20-crd-virtualmachine.yaml"

pip3 install -r "${repo_dir}/test/requirements.txt"

python3 "${repo_dir}/.ci/integration_test.py"
