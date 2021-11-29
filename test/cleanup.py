import argparse
import os

import test_util

TEST_NAMESPACE = "kube-system"


def cleanup(
    path_to_credentials_file: str,
    path_to_kubeconfig: str,
    test_namespace: str = TEST_NAMESPACE,
):
    k8s_helper, ip_helper, lb_helper = test_util._initialize_test_helpers(
        path_to_credentials_file=path_to_credentials_file,
        path_to_kubeconfig=path_to_kubeconfig,
    )
    print('Removing leftover publicipaddresses')
    k8s_helper.cleanup_publicip_custom_objects(namespace=test_namespace)
    print('Removing leftover virtualmachines')
    k8s_helper.cleanup_vm_custom_objects(namespace=test_namespace)
    print('Removing service finalizers')
    k8s_helper.remove_service_finalizers()
    print('Removing node finalizers')
    k8s_helper.remove_node_finalizers()


def _parse_args():
    parser = argparse.ArgumentParser(formatter_class=argparse.ArgumentDefaultsHelpFormatter)
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
        help='Namespace in the cluster in which the test will create publicipaddress objects.',
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

    cleanup(
        path_to_kubeconfig=path_to_kubeconfig,
        path_to_credentials_file=args.credentials_path,
        test_namespace=args.test_namespace,
    )
