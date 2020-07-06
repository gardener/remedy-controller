#!/usr/bin/env python3

import argparse
import json
import os
import sys
import subprocess
import tempfile
import concurrent.futures

import yaml

import ci.util
import landscape_setup.utils

own_dir = os.path.abspath(os.path.dirname(__file__))
repo_dir = os.path.abspath(os.path.join(own_dir, os.pardir))

sys.path.insert(0, os.path.join(repo_dir, 'test'))

import pubip_remedy_test as pubip_test # noqa
import failed_vm_test as vm_test # noqa
import test_util # noqa

HELM_CHART_NAME = 'remedy-controller-azure'
HELM_CHART_DEPLOYMENT_NAMESPACE = 'default'

VM_TEST_REQUIRED_ATTEMPTS = 4


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--kubeconfig-name', default='remedy-test-cluster')
    parser.add_argument('--credentials-config-name', default='integration_test')
    parsed = parser.parse_args()

    cfg_factory = ci.util.ctx().cfg_factory()
    kubernetes_config = cfg_factory.kubernetes(parsed.kubeconfig_name)
    test_credentials = cfg_factory._cfg_element(
        cfg_type_name='remedy_test',
        cfg_name=parsed.credentials_config_name,
    )

    with tempfile.NamedTemporaryFile(mode='w', delete=False) as kubeconfig_file:
        yaml.safe_dump(kubernetes_config.kubeconfig(), kubeconfig_file)
        kubeconfig_path = os.path.abspath(kubeconfig_file.name)
        os.environ['KUBECONFIG'] = kubeconfig_path

    # vm failer expects the credentials at one special location. TODO: Remove this once its adjusted
    expected_dir = os.path.join(repo_dir, 'dev')
    expected_file_path = os.path.join(expected_dir, 'credentials.yaml')
    os.mkdir(expected_dir)
    with open(expected_file_path, mode='w') as f:
        yaml.safe_dump(test_credentials.raw, f)

    with tempfile.NamedTemporaryFile(mode='w', delete=False) as credentials_file:
        yaml.safe_dump(test_credentials.raw, credentials_file)
        credentials_path = os.path.abspath(credentials_file.name)

    with open(os.path.join(repo_dir, 'VERSION')) as version_file:
        version = version_file.read()

    chart_dir = os.path.join(repo_dir, 'charts', HELM_CHART_NAME)
    values = create_helm_values(chart_dir, version, credentials_path)

    # TODO: Uncomment as soon as the python client for 1.16 is released & included
    # apply_crd(path_to_kubeconfig=kubeconfig_path)

    landscape_setup.utils.execute_helm_deployment(
        kubernetes_config,
        HELM_CHART_DEPLOYMENT_NAMESPACE,
        chart_dir,
        HELM_CHART_NAME,
        values,
    )

    with concurrent.futures.ThreadPoolExecutor(max_workers=2) as executor:
        pubip_future = executor.submit(
            fn=pubip_test.run_test,
            path_to_credentials_file=credentials_path,
            path_to_kubeconfig=kubeconfig_path,
            test_namespace=HELM_CHART_DEPLOYMENT_NAMESPACE,
        )
        failed_vm_future = executor.submit(
            fn=vm_test.run_test,
            path_to_credentials_file=credentials_path,
            path_to_kubeconfig=kubeconfig_path,
            required_attempts=VM_TEST_REQUIRED_ATTEMPTS,
            check_interval=10,
            run_duration=360,
        )

    pubip_test_ok = False
    vm_test_ok = False

    try:
        pubip_test_ok = pubip_future.result()
        vm_test_ok = failed_vm_future.result()
    finally:
        uninstall_helm_deployment(
            kubernetes_config,
            HELM_CHART_DEPLOYMENT_NAMESPACE,
            HELM_CHART_NAME,
        )
    if not pubip_test_ok or not vm_test_ok:
        exit(1)


def apply_crd(path_to_kubeconfig):
    k8s_client = test_util.KubernetesHelper(path_to_kubeconfig)
    with open(os.path.join('..', 'example', '20-crd-publicipaddress.yaml')) as crd_file:
        crd = yaml.safe_load(crd_file.read())
    k8s_client.create_custom_resource_definition(crd)


def create_helm_values(chart_dir, version, path_to_credentials_file):

    with open(os.path.join(path_to_credentials_file)) as credentials_file:
        credentials = yaml.safe_load(credentials_file)

    with open(os.path.join(chart_dir, 'values.yaml')) as values_file:
        values = yaml.safe_load(values_file)

    values['image']['tag'] = version
    values['cloudProviderConfig'] = json.dumps(credentials)

    # lower default values in order to speed up failed-vm-test
    values['config']['azure']['failedVMRemedy']['requeueInterval'] = '30s'
    values['config']['azure']['failedVMRemedy']['maxReapplyAttempts'] = VM_TEST_REQUIRED_ATTEMPTS

    # set the node selector so that the remedy-controller _wont_ run on the nodes that
    # will be failed
    values['nodeSelector'] = {'worker.garden.sapcloud.io/group': 'test-nodes'}

    return values


def uninstall_helm_deployment(
    kubernetes_config,
    namespace: str,
    release_name: str,
):
    helm_executable = landscape_setup.utils.ensure_helm_setup()

    KUBECONFIG_FILE_NAME = "kubecfg"

    # prepare subprocess args using relative file paths for the values files
    subprocess_args = [
        helm_executable,
        "uninstall",
        release_name,
        "--namespace",
        namespace,
    ]

    helm_env = os.environ.copy()
    helm_env['KUBECONFIG'] = KUBECONFIG_FILE_NAME

    # create temp dir containing all previously referenced files
    with tempfile.TemporaryDirectory() as temp_dir:

        with open(os.path.join(temp_dir, KUBECONFIG_FILE_NAME), 'w') as f:
            yaml.dump(kubernetes_config.kubeconfig(), f)

        # run helm from inside the temporary directory so that the prepared file paths work
        subprocess.run(subprocess_args, check=True, cwd=temp_dir, env=helm_env)


if __name__ == '__main__':
    main()
