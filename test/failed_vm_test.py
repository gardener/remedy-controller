import argparse
import os
import random
import subprocess
import time

import test_util

own_dir = os.path.abspath(os.path.dirname(__file__))
repo_dir = os.path.abspath(os.path.join(own_dir, os.pardir))
abs_repo_root_path = os.path.abspath(repo_dir)

FAIL_WORKER_GROUP_NAME = 'fail-me'
TEST_NAMESPACE = 'kube-system'
REQUIRED_ATTEMPTS = 5
CHECK_INTERVAL = 10
RUN_DURATION = 600


def run_test(
    path_to_credentials_file: str,
    path_to_kubeconfig: str,
    required_attempts: int = REQUIRED_ATTEMPTS,
    check_interval: int = CHECK_INTERVAL,
    run_duration: int = RUN_DURATION,
    fail_worker_group_name: str = FAIL_WORKER_GROUP_NAME,
    test_namespace: str = TEST_NAMESPACE,
) -> bool:
    k8s_helper, _, _ = test_util._initialize_test_helpers(
        path_to_credentials_file=path_to_credentials_file,
        path_to_kubeconfig=path_to_kubeconfig,
    )

    vm_name = random.choice(k8s_helper.node_names(worker_group=fail_worker_group_name))

    if not vm_name:
        print(f'Could not find VM belonging to worker group "{fail_worker_group_name}"')
        return False

    _run_disturber(vm_name)

    current_time = time.monotonic()
    attempts = 0

    while time.monotonic() - current_time < run_duration:
        failed_ops = [
            failed_op for failed_op in k8s_helper.get_failed_operations(
                name=vm_name,
                namespace=test_namespace,
            )
            if failed_op.type is test_util.FailedOperationType.REAPPLY_VM
        ]
        if failed_ops:
            if len(failed_ops) != 1:
                raise RuntimeError('More than one failed VM-reapply-operation found.')
            op = failed_ops[0]
            attempts = op.attempts
        time.sleep(check_interval)

    if not attempts == required_attempts:
        print(
            'VM remediation test failed - did not find the expected amount of reapply attempts. '
            f'Found: {attempts}, expected: {required_attempts}'
        )
        return False
    return True


def _run_disturber(vm_name):
    subprocess.run(
        args=[
            'make',
            f'VM_NAME={vm_name}',
            '-C', abs_repo_root_path,
            'start-failedvm-simulator-azure',
        ],
    )


def _parse_args():
    parser = argparse.ArgumentParser(formatter_class=argparse.ArgumentDefaultsHelpFormatter)
    parser.add_argument(
        '--run-duration',
        dest='run_duration',
        type=int,
        help='How long (in seconds) the test should run',
        default=RUN_DURATION,
    )
    parser.add_argument(
        '--check-interval',
        dest='check_interval',
        type=int,
        help=(
            'Period of time (in seconds) for checking for an updated status on the virtualmachine '
            'custom resource.'
        ),
        default=CHECK_INTERVAL,
    )
    parser.add_argument(
        '--fail-worker-group-name',
        dest='fail_worker_group_name',
        type=str,
        help=(
            'Name of the Gardener worker-group of the VM that should be failed. Only one of the '
            'VMs in the worker-group will be failed (chosen at random).'
        ),
        default=FAIL_WORKER_GROUP_NAME,
    )
    parser.add_argument(
        '--required-attempts',
        dest='required_attempts',
        type=int,
        help='Number of retry-attempts that must be detected to consider this test-run a success.',
        default=REQUIRED_ATTEMPTS,
    )
    parser.add_argument(
        '--kubeconfig-path',
        dest='kubeconfig_path',
        type=str,
        help='Path to kubeconfig file. Will try to use $KUBECONFIG env var if not given.',
        required=False
    )
    parser.add_argument(
        '--credentials-path',
        dest='credentials_path',
        type=str,
        help='Path to credentials file.',
        required=True
    )
    parser.add_argument(
        '--test-namespace',
        dest='test_namespace',
        type=str,
        help='Namespace in the cluster in which the test will look for virtualmachine objects.',
        default=TEST_NAMESPACE,
    )
    return parser.parse_args()


if __name__ == '__main__':
    args = _parse_args()

    if not args.kubeconfig_path:
        print("'--kubeconfig-path' not set, defaulting to $KUBECONFIG env var")
        if 'KUBECONFIG' not in os.environ:
            print("'KUBECONFIG' env var must be set.")
            exit(1)
        path_to_kubeconfig = os.environ['KUBECONFIG']
    else:
        path_to_kubeconfig = args.kubeconfig_path

    ok = run_test(
        path_to_kubeconfig=path_to_kubeconfig,
        path_to_credentials_file=args.credentials_path,
        test_namespace=args.test_namespace,
        required_attempts=args.required_attempts,
        check_interval=args.check_interval,
        run_duration=args.run_duration,
        fail_worker_group_name=args.fail_worker_group_name,
    )
    if not ok:
        exit(1)
