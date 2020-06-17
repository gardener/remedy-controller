import kubernetes
import os
import pprint
import random
import string

import kubernetes.client
import yaml

from azure.common.credentials import ServicePrincipalCredentials
from azure.mgmt.network import NetworkManagementClient
from kubernetes.config.kube_config import KubeConfigLoader

from kubernetes.client import (
    V1ObjectMeta,
    V1Service,
    V1ServicePort,
    V1ServiceSpec,
)

# Defines the prefixes for all generated resources in azure
LBR_NAME_PREFIX = 'lbr-'
FIP_NAME_PREFIX = 'fip-'
IP_NAME_PREFIX = 'ip-'
PROBE_NAME_PREFIX = 'prb-'

PUBIP_RESOURCE_API_GROUP = 'azure.remedy.gardener.cloud'
PUBIP_RESOURCE_VERSION = 'v1alpha1'


def random_str(prefix=None, length=10):
    '''Create a random string of given length, optionally with a given prefix

    The length of the generated string will always match the given value, no matter what prefix is
    given.
    '''
    if prefix:
        length -= len(prefix)
    else:
        prefix = ''
    return prefix + ''.join(random.choice(string.ascii_lowercase) for _ in range(length))


class IP:
    '''Helper class to keep track of created IPs and the random suffix used when creating them
    '''
    def __init__(self, name, id, suffix, ip_address):
        self._name = name
        self._id = id
        self._suffix = suffix
        self._ip_address = ip_address

    def name(self):
        return self._name

    def id(self):
        return self._id

    def random_string_suffix(self):
        return self._suffix

    def ip_address(self):
        return self._ip_address


class PublicIpHelper:
    '''Helper class for interacting with Azure's public ip resources
    '''
    def __init__(
        self,
        client,  # TODO: Azure calls this "PublicIpOperations". Maybe follow suit?
        resource_group_name,
        region,
    ):
        self.client = client.public_ip_addresses
        self.resource_group_name = resource_group_name
        self.region = region

    def _ip_parameters(self):
        return {
            'sku': {
                'name': 'Standard',  # TODO: Make configurable. Should only be 'Standard' for us,
                                     # though
            },
            'location': self.region,
            'public_ip_allocation_method': 'static',
            'idle_timeout_in_minutes': 4,
        }

    def create_public_ips(self, count: int):
        return [self.create_public_ip(random_str(length=12)) for _ in range(count)]

    def create_public_ip(self, ip_address_name_suffix):
        ip_address_name = f'{IP_NAME_PREFIX}{ip_address_name_suffix}'
        async_publicip_creation = self.client.create_or_update(
            resource_group_name=self.resource_group_name,
            public_ip_address_name=f'{ip_address_name}',
            parameters=self._ip_parameters()
        )
        ip = async_publicip_creation.result()
        return IP(name=ip.name, id=ip.id, ip_address=ip.ip_address, suffix=ip_address_name_suffix)

    def delete_public_ip(self, ip):
        async_publicip_deletion = self.client.delete(
            resource_group_name=self.resource_group_name,
            public_ip_address_name=ip.name(),
        )
        # Wait for completion
        async_publicip_deletion.result()

    def orphaned_public_ips(self):
        '''Returns a list containing all orphaned IPs in the resource group this helper targets

        Note: Any public ip that is found matching the naming scheme 'ip-<rnd string>' is considered
        orphaned by this function.
        '''
        paged_response = self.client.list(
            resource_group_name=self.resource_group_name,
        )

        def extract_rnd_suffix(ip_name):
            return ip_name.split('-')[-1]

        return [
            IP(ip.name, ip.id, extract_rnd_suffix(ip.name), ip.ip_address)
            for ip in paged_response
            if ip.name.startswith(IP_NAME_PREFIX)
        ]

    def clean_up_public_ips(self):
        for ip in self.orphaned_public_ips():
            self.delete_public_ip(ip)


class LoadBalancerHelper:
    '''Helper class for interacting with Azure load balancers
    '''
    def __init__(
        self,
        client,
        resource_group_name,
        subscription_id,
        region,
    ):
        self.client = client.load_balancers
        self.region = region
        self.subscription_id = subscription_id
        self.resource_group_name = resource_group_name
        # Load Balancer is named like the resource group
        self.load_balancer_name = resource_group_name

    def _get_current_lb_configuration(self):
        return self.client.get(
            resource_group_name=self.resource_group_name,
            load_balancer_name=self.load_balancer_name,
        )

    def _future_fip_id(self, fip_config_name):
        '''Calculate the fip id that will be created for the given fip config.

        Required when adding a new fip, as the (not yet assigned) fip id is required. So
        calculate it.
        '''
        return (
            f'/subscriptions/{self.subscription_id}'
            f'/resourceGroups/{self.resource_group_name}'
            '/providers/Microsoft.Network'
            f'/loadBalancers/{self.load_balancer_name}'
            f'/frontendIPConfigurations/{fip_config_name}'
        )

    def _new_frontend_ip_config_for_public_ip(
        self,
        public_ip: IP,
    ):
        '''Create a new frontend ip config.

        Returns the (future) id of the config and the config itself
        '''
        fip_config_name = f'{FIP_NAME_PREFIX}{public_ip.random_string_suffix()}'
        fip_config = {
            'name': fip_config_name,
            'private_ip_allocation_method': 'Dynamic',
            'public_ip_address': {
                'id': public_ip.id(),
            },
        }
        return self._future_fip_id(fip_config_name), fip_config

    def _future_probe_id(self, probe_name):
        # see _future_fip_id
        return (
            f'/subscriptions/{self.subscription_id}'
            f'/resourceGroups/{self.resource_group_name}'
            '/providers/Microsoft.Network/loadBalancers/'
            f'{self.load_balancer_name}/probes/{probe_name}'
        )

    def _new_health_probe_config(self, probe_name_suffix):
        # see _new_frontend_ip_config_for_public_ip
        probe_config_name = f'{PROBE_NAME_PREFIX}{probe_name_suffix}'
        health_probe_config = {
            'name': probe_config_name,
            'protocol': 'Tcp',
            'port': 32000 + random.randint(0, 2500),  # TODO: Check valid range
            'interval_in_seconds': 60,
            'number_of_probes': 4,  # Unhealthy threshold
        }
        return self._future_probe_id(probe_config_name), health_probe_config

    def _backup_pool_id(self, address_pool_name):
        # see _future_fip_id
        return (
            f'/subscriptions/{self.subscription_id}'
            f'/resourceGroups/{self.resource_group_name}'
            '/providers/Microsoft.Network'
            f'/loadBalancers/{self.load_balancer_name}'
            f'/backendAddressPools/{address_pool_name}'
        )

    def _create_lb_rule_for_public_ip(
        self,
        public_ip: IP,
    ):
        '''Create a new load balancing rule for the given public ip

        Will also configure a frontend ip config and a health probe.

        Returns a triple of (load balancer config, frontend ip config, health probe config)
        '''
        load_balancing_rule_name = f'{LBR_NAME_PREFIX}{public_ip.random_string_suffix()}'
        load_balancing_port = random.randint(2000, 4000) # TODO: check range

        # If we want to create a new LB rule, we also need to create new fip and probe configs and
        # add their (not yet existing) IDs on Azure.
        frontend_ip_config_id, frontend_ip_config = self._new_frontend_ip_config_for_public_ip(
            public_ip=public_ip,
        )
        backend_address_pool_id = self._backup_pool_id(
            address_pool_name=self.resource_group_name,
        )
        health_probe_config_id, health_probe_config = self._new_health_probe_config(
            probe_name_suffix=public_ip.random_string_suffix(),
        )
        load_balancing_rule_config = {
            'name': load_balancing_rule_name,
            'protocol': 'tcp',
            'frontend_port': load_balancing_port,
            'backend_port': load_balancing_port,
            'idle_timeout_in_minutes': 4,
            'enable_floating_ip': True,
            'load_distribution': 'Default',
            'frontend_ip_configuration': {
                'id': frontend_ip_config_id,
            },
            'backend_address_pool': {
                'id': backend_address_pool_id,
            },
            'probe': {
                'id': health_probe_config_id,
            },
        }
        return load_balancing_rule_config, frontend_ip_config, health_probe_config

    def add_rules_for_public_ips(self, public_ips: list):
        lb_cfg = self._get_current_lb_configuration()
        fip_configs = lb_cfg.frontend_ip_configurations
        lb_rules = lb_cfg.load_balancing_rules
        probes = lb_cfg.probes

        for ip in public_ips:
            # create a new lb rule config for the given ip. This will also require creation
            # of new fip and probe configs.
            lb_rule_config, fip_config, probe_config = self._create_lb_rule_for_public_ip(ip)
            fip_configs.append(fip_config)
            lb_rules.append(lb_rule_config)
            probes.append(probe_config)

        try:
            # NOTE: All parameters are required. Parameters left out are interpreted as a removal of
            # said parameter's values.
            self.client.create_or_update(
                resource_group_name=self.resource_group_name,
                load_balancer_name=self.load_balancer_name,
                parameters={
                    'location': self.region,
                    'sku': lb_cfg.sku,
                    'frontend_ip_configurations': fip_configs,
                    'backend_address_pools': lb_cfg.backend_address_pools,
                    'load_balancing_rules': lb_rules,
                    'probes': probes,
                    'inbound_nat_rules': lb_cfg.inbound_nat_rules,
                    'outbound_rules': lb_cfg.outbound_rules,
                    'type': lb_cfg.type
                },
            )
        except:
            return False

        return True

    def delete_rules_for_public_ips(self, public_ips: list):
        # simply remove all lb rules, fips and probes sharing the same randomly generated suffix.
        lb_cfg = self._get_current_lb_configuration()
        fip_configs = lb_cfg.frontend_ip_configurations
        lb_rules = lb_cfg.load_balancing_rules
        probes = lb_cfg.probes

        ip_suffixes = [ip.random_string_suffix() for ip in public_ips]

        # The following reads as 'A list of those fip configs in "fip_configs" that do not have any
        # of the suffixes in "ip_suffixes" in their name'. Those are the ones we need to keep.
        fip_configs = [
            f
            for f in fip_configs
            if not any(s in f.name for s in ip_suffixes)
        ]
        lb_rules = [
            l for l in lb_rules
            if not any(s in l.name for s in ip_suffixes)
        ]
        probes = [
            p for p in probes
            if not any(s in p.name for s in ip_suffixes)
        ]

        self.client.create_or_update(
                resource_group_name=self.resource_group_name,
                load_balancer_name=self.load_balancer_name,
                parameters={
                    'location': self.region,
                    'sku': lb_cfg.sku,
                    'frontend_ip_configurations': fip_configs,
                    'backend_address_pools': lb_cfg.backend_address_pools,
                    'load_balancing_rules': lb_rules,
                    'probes': probes,
                    'inbound_nat_rules': lb_cfg.inbound_nat_rules,
                    'outbound_rules': lb_cfg.outbound_rules,
                    'type': lb_cfg.type
                },
            )

    def _orphaned_load_balancer_resources(self):
        lb_cfg = self._get_current_lb_configuration()
        fip_configs = lb_cfg.frontend_ip_configurations
        lb_rules = lb_cfg.load_balancing_rules
        probes = lb_cfg.probes
        return (
            [r for r in lb_rules if r.name.startswith(LBR_NAME_PREFIX)],
            [f for f in fip_configs if f.name.startswith(FIP_NAME_PREFIX)],
            [p for p in probes if p.name.startswith(PROBE_NAME_PREFIX)],
        )

    def check_for_orphaned_resources(self, raise_on_leak=False):
        '''Checks for orphaned resources and pretty prints the findings.

        If 'raise_on_leak' is given this will raise a RuntimeError if orphaned resources are found
        '''
        rules, fips, probes = self._orphaned_load_balancer_resources()

        leaked_lb_rules = len(rules)
        leaked_fips = len(fips)
        leaked_probes = len(probes)

        print('---')
        print(f'orphaned load balancing rules: {leaked_lb_rules}')
        print(f'orphaned floating ip configs: {leaked_fips}')
        print(f'orphaned health probes: {leaked_probes}')
        print('---')
        if raise_on_leak and (leaked_lb_rules != 0 or leaked_fips != 0 or leaked_probes != 0):
            raise RuntimeError(
                'Resource leaks detected: '
                f'lb rules: {leaked_lb_rules} - '
                f'fip configs: {leaked_fips} - '
                f'health probes: {leaked_probes}'
        )

    def remove_orphaned_rules(self):
        lb_cfg = self._get_current_lb_configuration()

        fip_configs = lb_cfg.frontend_ip_configurations
        lb_rules = lb_cfg.load_balancing_rules
        probes = lb_cfg.probes

        orphaned_rules, orphaned_fips, orphaned_probes = self._orphaned_load_balancer_resources()

        fip_configs = [f for f in fip_configs if f not in orphaned_fips]
        lb_rules = [l for l in lb_rules if l not in orphaned_rules]
        probes = [p for p in probes if p not in orphaned_probes]

        self.client.create_or_update(
            resource_group_name=self.resource_group_name,
            load_balancer_name=self.load_balancer_name,
            parameters={
                'location': self.region,
                'sku': lb_cfg.sku,
                'frontend_ip_configurations': fip_configs,
                'backend_address_pools': lb_cfg.backend_address_pools,
                'load_balancing_rules': lb_rules,
                'probes': probes,
                'inbound_nat_rules': lb_cfg.inbound_nat_rules,
                'outbound_rules': lb_cfg.outbound_rules,
                'type': lb_cfg.type
            },
        )


class KubernetesHelper:
    '''Helper class for kubernetes-related operations
    '''
    def __init__(
        self,
        path_to_kubeconfig: str,
    ):
        api_client = self._create_kubernetes_api_client(path_to_kubeconfig)
        self.core_api = kubernetes.client.CoreV1Api(api_client)
        self.custom_objects_api = kubernetes.client.CustomObjectsApi(api_client)

        # ApiextensionsV1Api is not yet included in the latest k8s-client release
        # self.api_extensions_api = kubernetes.client.ApiextensionsV1Api(api_client)

    def _load_kubeconfig(self, path_to_kubeconfig: str):
        if not os.path.isfile(path_to_kubeconfig):
            print(f"No file found at '{path_to_kubeconfig}'")
            exit(1)

        with open(path_to_kubeconfig, 'r') as kubeconfig_file:
            kubeconfig = yaml.safe_load(kubeconfig_file.read())

        k8s_config = kubernetes.client.Configuration()
        cfg_loader = KubeConfigLoader(kubeconfig)
        cfg_loader.load_and_set(k8s_config)
        return k8s_config

    def _create_kubernetes_api_client(self, path_to_kubeconfig: str):
        config = self._load_kubeconfig(path_to_kubeconfig)
        return kubernetes.client.ApiClient(configuration=config)

    def create_service(
        self,
        service_name: str,
        namespace: str ='default',
        port: int = 80,
        protocol: str = 'TCP',
        service_type: str = 'LoadBalancer',
        target_port: int = 8080,
    ):
        self.core_api.create_namespaced_service(
                namespace=namespace,
                body=V1Service(
                    kind='Service',
                    metadata=V1ObjectMeta(
                        name=service_name,
                    ),
                    spec=V1ServiceSpec(
                        type=service_type,
                        ports=[
                            V1ServicePort(protocol=protocol, port=port, target_port=target_port),
                        ],
                        selector={'app': service_name},
                        session_affinity='None',
                    ),
                )
            )

    def create_test_services(
        self,
        count: int,
        namespace: str = 'default',
    ):
        '''Create a given amount of simple test services in the given namespace.
        '''
        service_names = {random_str(prefix='svc-') for _ in range(count)}
        for name in service_names:
            self.create_service(
                service_name=name,
                namespace=namespace,
            )
        return service_names

    def cleanup_test_services(
        self,
        service_names: list = [],
        namespace: str = 'default',
    ):
        '''Clean up services created by the test.

        If `service_names` is given, _only_ services with names whose name is contained in the given
        list will be deleted.
        '''
        svc_list = self.core_api.list_namespaced_service(namespace=namespace)
        for svc in svc_list.items:
            svc_name = svc.metadata.name
            if service_names and svc_name not in service_names:
                continue
            if svc_name.startswith('svc-'):
                self.core_api.delete_namespaced_service(name=svc_name, namespace=namespace)

    def _create_pubip_resource(
        self,
        resource_name: str,
        namespace: str,
        ip_address: str,
    ):
        self.custom_objects_api.create_namespaced_custom_object(
            group=PUBIP_RESOURCE_API_GROUP,
            version=PUBIP_RESOURCE_VERSION,
            namespace=namespace,
            plural='publicipaddresses',
            body={
                'apiVersion': f'{PUBIP_RESOURCE_API_GROUP}/{PUBIP_RESOURCE_VERSION}',
                'kind': 'PublicIPAddress',
                'metadata': {'name': resource_name},
                'spec': {'ipAddress': ip_address},
            },
        )

    def create_publicip_custom_objects(
        self,
        ips: list,
        namespace: str = 'default',
    ):
        for ip in ips:
            self._create_pubip_resource(
                resource_name=f'pubip-{ip.name()}',
                namespace=namespace,
                ip_address=ip.ip_address(),
            )

    def delete_publicip_custom_objects(
        self,
        ips: list,
        namespace: str = 'default',
    ):
        for ip in ips:
            self.custom_objects_api.delete_namespaced_custom_object(
                group=PUBIP_RESOURCE_API_GROUP,
                version=PUBIP_RESOURCE_VERSION,
                namespace=namespace,
                plural='publicipaddresses',
                name=f'pubip-{ip.name()}',
            )

    def cleanup_publicip_custom_objects(
        self,
        namespace: str = 'default',
    ):
        response = self.custom_objects_api.list_namespaced_custom_object(
                    group=PUBIP_RESOURCE_API_GROUP,
                    version=PUBIP_RESOURCE_VERSION,
                    namespace=namespace,
                    plural='publicipaddresses',
                )
        if not 'items' in response:
            return

        # remove finalizers from all found objects
        for item in response['items']:
            if 'finalizers' in item['metadata']:
                item['metadata']['finalizers'] = []
                self.custom_objects_api.patch_namespaced_custom_object(
                    group=PUBIP_RESOURCE_API_GROUP,
                    version=PUBIP_RESOURCE_VERSION,
                    namespace=namespace,
                    plural='publicipaddresses',
                    name=item['metadata']['name'],
                    body=item,
                )

        # delete
        for item in response['items']:
            self.custom_objects_api.delete_namespaced_custom_object(
                group=PUBIP_RESOURCE_API_GROUP,
                version=PUBIP_RESOURCE_VERSION,
                namespace=namespace,
                plural='publicipaddresses',
                name=item['metadata']['name'],
            )

    def create_custom_resource_definition(self, custom_resource_definition):
        self.api_extensions_api.create_custom_resource_definition(
            body=custom_resource_definition,
        )


def _initialize_test_helpers(path_to_credentials_file, path_to_kubeconfig):

    # Helper function that initializes all the helper classes in this file for the test
    # and returns them

    k8s_helper = KubernetesHelper(path_to_kubeconfig=path_to_kubeconfig)

    with open(path_to_credentials_file) as f:
        credentials_dict = yaml.safe_load(f)
        pprint.pprint(credentials_dict)

    subscription_id = credentials_dict['subscriptionId']
    resource_group_name = credentials_dict['resourceGroup']
    region = credentials_dict['location']
    credentials = ServicePrincipalCredentials(
        client_id=credentials_dict['aadClientId'],
        secret=credentials_dict['aadClientSecret'],
        tenant=credentials_dict['tenantId'],
    )

    network_client = NetworkManagementClient(credentials, subscription_id)

    ip_helper = PublicIpHelper(
        client=network_client,
        resource_group_name=resource_group_name,
        region=region,
    )
    lb_helper = LoadBalancerHelper(
        client=network_client,
        resource_group_name=resource_group_name,
        subscription_id=subscription_id,
        region=region,
    )

    return k8s_helper, ip_helper, lb_helper
