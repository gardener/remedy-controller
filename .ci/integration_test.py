#!/usr/bin/env python3

import json
import os
import sys
import subprocess
import tempfile
import concurrent.futures

import yaml

import model.kubernetes

own_dir = os.path.abspath(os.path.dirname(__file__))
repo_dir = os.path.abspath(os.path.join(own_dir, os.pardir))

sys.path.insert(0, os.path.join(repo_dir, 'test'))

import pubip_remedy_test as pubip_test # noqa
import failed_vm_test as vm_test # noqa
import test_util # noqa

HELM_CHART_NAME = 'remedy-controller-azure'
HELM_CHART_DEPLOYMENT_NAMESPACE = 'default'

VM_TEST_REQUIRED_ATTEMPTS = 4

KUBECONFIG_DIR = os.environ['TM_KUBECONFIG_PATH']


def main():
    kubeconfig_path = os.path.join(KUBECONFIG_DIR, 'shoot.config')
    os.environ['KUBECONFIG'] = kubeconfig_path
    test_credentials = credentials_from_environ()

    with open(kubeconfig_path, 'r') as f:
        kubeconfig = yaml.safe_load(f.read())
        kubernetes_config = model.kubernetes.KubernetesConfig(
            '',
            {'kubeconfig': kubeconfig}, # MUST be positional
        )

    # vm failer expects the credentials at one special location. TODO: Remove this once its adjusted
    expected_dir = os.path.join(repo_dir, 'dev')
    expected_file_path = os.path.join(expected_dir, 'credentials.yaml')
    os.mkdir(expected_dir)
    with open(expected_file_path, mode='w') as f:
        yaml.safe_dump(test_credentials, f)

    with tempfile.NamedTemporaryFile(mode='w', delete=False) as credentials_file:
        yaml.safe_dump(test_credentials, credentials_file)
        credentials_path = os.path.abspath(credentials_file.name)

    with open(os.path.join(repo_dir, 'VERSION')) as version_file:
        version = version_file.read()

    chart_dir = os.path.join(repo_dir, 'charts', HELM_CHART_NAME)
    values = create_helm_values(chart_dir, version, credentials_path)

    test_util.execute_helm_deployment(
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
            test_namespace=HELM_CHART_DEPLOYMENT_NAMESPACE,
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


def credentials_from_environ():
    return {
        'aadClientId': os.environ['CLIENT_ID'],
        'aadClientSecret': os.environ['CLIENT_SECRET'],
        'tenantId': os.environ['TENANT_ID'],
        'subscriptionId': os.environ['SUBSCRIPTION_ID'],
        'resourceGroup': f'shoot--it--{os.environ["SHOOT_NAME"]}',
        'location': os.environ['REGION'],
    }


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
    helm_executable = test_util.ensure_helm_setup()

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
