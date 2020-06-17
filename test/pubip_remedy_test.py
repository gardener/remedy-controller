import argparse
import os
import random
import threading
import time

import test_util


def run_test(
    path_to_credentials_file: str,
    path_to_kubeconfig: str,
    run_duration: int = 600,
    svc_min_sleep: int = 20,
    svc_max_sleep: int = 180,
    min_svc_count: int = 5,
    max_svc_count: int = 20,
    orphaned_ip_min_sleep: int = 60,
    orphaned_ip_max_sleep: int = 70,
    shutdown_grace_period: int = 900,
):
    k8s_helper, ip_helper, lb_helper = test_util._initialize_test_helpers(
        path_to_credentials_file=path_to_credentials_file,
        path_to_kubeconfig=path_to_kubeconfig,
    )
    print('Ensuring the cluster has no leftover resources from previous run')
    try:
        lb_helper.check_for_orphaned_resources(raise_on_leak=True)
    except RuntimeError:
        print('Found leaked resources from previous run. Cleaning up')
        # ensure that there are no leftover resources
        k8s_helper.cleanup_test_services()
        k8s_helper.cleanup_publicip_custom_objects()
        lb_helper.remove_orphaned_rules()
        ip_helper.clean_up_public_ips()

        lb_helper.check_for_orphaned_resources(raise_on_leak=True)

    # prepare functions that will run the actual tests
    def svc_creation_test_func(
        thread_name,
        run_duration,
        min_sleep,
        max_sleep,
        min_svc_count,
        max_svc_count
    ):
        current_time = time.monotonic()

        while time.monotonic() - current_time < run_duration:
            svc_count = random.randint(min_svc_count, max_svc_count)
            service_names = k8s_helper.create_test_services(count=svc_count)

            sleep_time = random.randint(min_sleep, max_sleep)
            time.sleep(sleep_time)

            k8s_helper.cleanup_test_services(service_names=service_names)

    def orphaned_ip_creation_test_func(thread_name, run_duration, min_sleep, max_sleep):
        current_time = time.monotonic()

        while time.monotonic() - current_time < run_duration:
            ips = ip_helper.create_public_ips(count=1)
            rule_created = lb_helper.add_rules_for_public_ips(ips)
            if rule_created:
                k8s_helper.create_publicip_custom_objects(ips)
                time.sleep(10)
                k8s_helper.delete_publicip_custom_objects(ips)
            else:
                # Creation of rules failed (probably due to the load balancer being busy). Print
                # warning and clean up the IPs created in this iteration.
                print(
                    'Failed to create load balancer rules for public IP. Please check the '
                    'loadbalancer if this issue persists.'
                )
                for ip in ips:
                    ip_helper.delete_public_ip(ip)

            time.sleep(random.randint(min_sleep, max_sleep))

    svc_creation_thread = threading.Thread(
        target=svc_creation_test_func,
        kwargs={
            'thread_name': 'service creation thread',
            'run_duration': run_duration,
            'min_svc_count': min_svc_count,
            'max_svc_count': max_svc_count,
            'min_sleep': svc_min_sleep,
            'max_sleep': svc_max_sleep,
        },
    )
    orphaned_ip_creation_thread = threading.Thread(
        target=orphaned_ip_creation_test_func,
        kwargs={
            'thread_name': 'orphaned resource creation thread',
            'run_duration': run_duration,
            'min_sleep': orphaned_ip_min_sleep,
            'max_sleep': orphaned_ip_max_sleep,
        },
    )

    # start threads running the functions
    print('Starting test')
    svc_creation_thread.start()
    orphaned_ip_creation_thread.start()

    svc_creation_thread.join()
    orphaned_ip_creation_thread.join()

    print(f'Waiting {shutdown_grace_period} seconds before shutdown')
    time.sleep(shutdown_grace_period)

    print('Done - Checking for orphaned resources')
    try:
        lb_helper.check_for_orphaned_resources(raise_on_leak=True)
    except RuntimeError:
        print('Found leaked resources. Cleaning up and failing.')
        # ensure that there are no leftover resources
        k8s_helper.cleanup_test_services()
        k8s_helper.cleanup_publicip_custom_objects()
        lb_helper.remove_orphaned_rules()
        ip_helper.clean_up_public_ips()
        exit(1)


def _parse_args():
    parser = argparse.ArgumentParser(formatter_class=argparse.ArgumentDefaultsHelpFormatter)
    parser.add_argument(
        '--run-duration',
        dest='run_duration',
        type=int,
        help='How long (in seconds) the test should run',
        default=600,
    )
    parser.add_argument(
        '--svc-min-sleep',
        dest='svc_min_sleep',
        type=int,
        help='Minimum amount of time (in seconds) between service creation and deletion.',
        default=20,
    )
    parser.add_argument(
        '--svc-max-sleep',
        dest='svc_max_sleep',
        type=int,
        help='Maximum amount of time (in seconds) between service creation and deletion.',
        default=180,
    )
    parser.add_argument(
        '--min-svc-count',
        dest='min_svc_count',
        type=int,
        help='Minimum number of services created in one run',
        default=5,
    )
    parser.add_argument(
        '--max-svc-count',
        dest='max_svc_count',
        type=int,
        help='Maximum number of services created in one run',
        default=20,
    )
    parser.add_argument(
        '--shutdown-grace-period',
        dest='shutdown_grace_period',
        type=int,
        help=(
            'Length of the final shutdown grace period (in seconds) before checking whether '
            'everything was cleaned up properly.'
        ),
        default=900,
    )
    parser.add_argument(
        '--orphaned-ip-min-sleep',
        dest='orphaned_ip_min_sleep',
        type=int,
        help='Minimum amount of time (in seconds) between orphaned ip creations.',
        default=60,
    )
    parser.add_argument(
        '--orphaned-ip-max-sleep',
        dest='orphaned_ip_max_sleep',
        type=int,
        help='Maximum amount of time (in seconds) between orphaned ip creations.',
        default=70,
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

    run_test(
        path_to_kubeconfig=path_to_kubeconfig,
        path_to_credentials_file=args.credentials_path,
        svc_min_sleep=args.svc_min_sleep,
        svc_max_sleep=args.svc_max_sleep,
        min_svc_count=args.min_svc_count,
        max_svc_count=args.max_svc_count,
        orphaned_ip_min_sleep=args.orphaned_ip_min_sleep,
        orphaned_ip_max_sleep=args.orphaned_ip_max_sleep,
        shutdown_grace_period=args.shutdown_grace_period,
        )
