# Gardener Remedy Controllers

Gardener remedy controllers are special controllers that attempt to detect and remedy specific issues on certain platforms. They are intended to be deployed as part of the Shoot control plane alongside other platform-specific control plane components, such as the cloud controller manager. Since issues and corresponding remedies are platform-specific, there are different remedy controllers for the different platforms. The following remedy controllers exist currently:

- [Azure remedy controller](cmd/remedy-controller-azure)

In addition, this repository hosts additional binaries to apply remedies in batch mode and simulate certain platform issues to enable easier testing of the remedy controllers.

## General Concepts

### Design Principles

Although remedy controllers may apply different remedies for different platforms, they all follow similar design principles, outlined below.

- The remedy controller watches certain Kubernetes resources, e.g. services or nodes, in a _target cluster_. It is only interested in certain changes, for example a public IP address is added to a service, or a node becomes unreachable.
- When such a change is detected, it is reconciled by creating, updating, or deleting a special custom resource designed to track the corresponding platform resource, e.g. `PublicIPAddress` for public IP addresses, or `VirtualMachine` for virtual machines. This resource is created in a _control cluster_ that may be different from the target cluster. A finalizer is put on the original resource to make sure it can't be deleted unless the deletion has been properly reconciled by the remedy controller.
- As part of the creation, update, or deletion of the custom resource mentioned above, the remedy controller performs special actions to detect, and if needed correct, issues with the corresponding platform resource, e.g. a public IP address that still exists after its corresponding service has been deleted is considered to be orphaned and is therefore deleted by the controller.

### Handling Platform Rate Limits

Remedy controllers perform platform read and write operations only when needed, and retry such operations if they fail with backoff and for a limited number of times, to prevent it from exhausting platform rate limits due to infinite frequent retries. All time intervals and the maximum number of attempts are configurable via the [controller configuration](#configuration).

### Metrics and Alerts

Remedy controllers expose metrics for successfully applied remedies, and the number of platform read and write operations. For some remedies, they also expose metrics that allow raising an alert in case the remedy has not been applied successfully after the configured number of retries.

### Deployment Options

As mentioned above, the remedy controller watches Kubernetes resources in a _target cluster_, but manages custom tracking resources in a _control cluster_. These 2 clusters can be the same or different.

- When using the provided [Helm charts](charts), the cluster where the remedy controller is deployed is both the target and the control cluster. This deployment option is suitable for testing, or when using the controller to target a cluster not managed by Gardener.
- In a Gardener setup, the remedy controller is deployed to the Seed cluster as part of the Shoot control plane by the corresponding platform extension. In this case, the target cluster is the Shoot and the control cluster is the Seed.

## Features

### Azure Remedy Controller

#### Azure Remedies

##### Cleanup orphaned public IP addresses

In some cases, public IPs of services of type `LoadBalancer` are not properly deleted from Azure when the corresponding service is deleted. This may lead to issues as the Azure public IP quotas can gradually become exhausted. The Azure remedy controller tracks Azure public IPs of `LoadBalancer` services via custom `PublicIPAddress` resources and makes sure they are cleaned up properly. If such an address is not deleted within a configurable grace period after the corresponding service has been deleted, it is removed from the load balancer and deleted by the controller.

##### Reapply failed VMs

In some cases, due to certain race conditions, an Azure virtual machine can reach a `Failed` provisioning state. Even though in most cases such VMs are then deleted and replaced by the Machine Controller Manager, sometimes this also fails. The Azure remedy controller tracks Azure virtual machines of Kubernetes nodes via custom `VirtualMachine` resources and if a node is detected as not ready or unreachable, checks if the virtual machine has a `Failed` provisioning state, and reapplies the virtual machine spec if this is the case. This sometimes fixes the virtual machine and makes the Kubernetes node ready and reachable again.

#### Metrics and alerts

The Azure remedy controller exposes the following custom Prometheus metrics:

| Metric                                   | Type    | Description                                |
| ---------------------------------------- | ------- | ------------------------------------------ |
| `cleaned_azure_public_ips_total`         | Counter | Number of cleaned Azure public IPs         |
| `reapplied_azure_virtual_machines_total` | Counter | Number of reapplied Azure virtual machines |
| `azure_read_requests_total`              | Counter | Number of Azure read requests              |
| `azure_write_requests_total`             | Counter | Number of Azure write requests             |

## Deploying to Kubernetes

1. Clone this repository. Unless you are developing in the project, be sure to checkout to a [tagged release](https://github.com/gardener/remedy-contoller/releases).

   ```bash
   git clone https://github.com/gardener/remedy-contoller
   cd remedy-contoller
   git checkout <tag>
   ```

2. Prepare a `credentials.yaml` file with the correct platform credentials and configuration. For Azure, this file should have the following format:

   ```yaml
   aadClientId: "<client id>"
   aadClientSecret: "<client secret>"
   tenantId: "<tenant id>"
   subscriptionId: "<subscription id>"
   resourceGroup: "<resource group name>"
   location: "<azure region name>"
   ```

3. Ensure that the CRDs for custom resources used by the remedy controller for your platform are deployed to the cluster. For Azure, these CRDs are [example/20-crd-publicipaddress.yaml](example/20-crd-publicipaddress.yaml) and [example/20-crd-virtualmachine.yaml](example/20-crd-virtualmachine.yaml).

4. Create the namespace to deploy the remedy controller for your platform.

   ```bash
   kubectl create namespace remedy-controller-azure
   ```

5. Deploy the remedy controller for your platform and all additional objects it requires to the cluster by applying the corresponding Helm chart, passing the `credentials.yaml` file you prepared above, and other custom values if needed. For Azure, this is [charts/remedy-controller-azure](charts/remedy-controller-azure).

   ```bash
   helm upgrade -i remedy-controller-azure charts/remedy-controller-azure -n remedy-controller-azure \
     --set-file cloudProviderConfig=dev/credentials.yaml
   ```

   **Note:** The Helm charts are designed to be applied with Helm 3. To install Helm, you can execute `make install-requirements`.

## Configuration

### Command Line Options

The remedy controllers for all platforms can be configured using the following command line options:

| Option                          | Type    | Description                                                                                                                                   |
| ------------------------------- | ------- | --------------------------------------------------------------------------------------------------------------------------------------------- |
| `--config-file`                 | string  | The path to the controller manager [configuration file](#configuration-file).                                                                 |
| `--infrastructure-config`       | string  | The path to the infrastructure credentials and configuration file.                                                                            |
| `--leader-election`             |         | Whether to use leader election or not when running this controller manager. (default true)                                                    |
| `--leader-election-id`          | string  | The leader election id to use. (default "remedy-controller-azure-leader-election")                                                            |
| `--leader-election-namespace`   | string  | The namespace to do leader election in. (default "garden")                                                                                    |
| `--kubeconfig`                  | string  | The path to a kubeconfig file. Only required if out-of-cluster.                                                                               |
| `--master`                      | string  | The address of the Kubernetes API server. Overrides any value in `kubeconfig`. Only required if out-of-cluster.                               |
| `--namespace`                   | string  | The namespace to watch objects in. (default "kube-system")                                                                                    |
| `--metrics-bind-address`        | string  | The TCP address that the controller should bind to for serving prometheus metrics. (default ":6000")                                          |
| `--disable-controllers`         | strings | Comma-separated list of controllers to disable.                                                                                               |
| `--target-kubeconfig`           | string  | The path to a kubeconfig file for the target cluster. Only required if out-of-cluster.                                                        |
| `--target-master`               | string  | The address of the Kubernetes API server for the target cluster. Overrides any value in `target-kubeconfig`. Only required if out-of-cluster. |
| `--target-namespace`            | string  | The namespace to watch objects in, for the target cluster. (default all namespaces)                                                           |
| `--target-metrics-bind-address` | string  | The TCP address that the controller should bind to for serving prometheus metrics, for the target cluster. (default ":6001")                  |
| `--target-disable-controllers`  | string  | Comma-separated list of controllers to disable for the target cluster.                                                                        |

#### Azure-specific

The Azure remedy controller has the following additional command line options:

| Option                                        | Type | Description                                                                                      |
| --------------------------------------------- | ---- | ------------------------------------------------------------------------------------------------ |
| `--service-max-concurrent-reconciles`         | int  | The maximum number of concurrent reconciliations for the service controller. (default 5)         |
| `--node-max-concurrent-reconciles`            | int  | The maximum number of concurrent reconciliations for the node controller. (default 5)            |
| `--publicipaddress-max-concurrent-reconciles` | int  | The maximum number of concurrent reconciliations for the publicipaddress controller. (default 5) |
| `--virtualmachine-max-concurrent-reconciles`  | int  | The maximum number of concurrent reconciliations for the virtualmachine controller. (default 5)  |

### Configuration File

The remedy controllers accept a configuration file in YAML format via the `--config-file` command line option. This file has the format described in the [configuration reference documentation](hack/api-reference/config.md) and used in this [example](example/00-config.yaml).

## Local Development and Testing

To run the remedy controller for a certain platform locally on your machine:

1. Prepare a `credentials.yaml` file with the correct platform credentials and configuration and copy it to `dev/credentials.yaml`. For the correct format of this file, see [Deploying to Kubernetes](#deploying-to-kubernetes).

2. Ensure that the `KUBECONFIG` environment variable points to a kubeconfig file for an existing Azure cluster.

3. Execute `make start-<platform>`. For Azure, execute `make start-azure`.

To run static code checks and unit tests:

1. Execute `make install-requirements` once to ensure that all requirements are properly installed.

2. Execute `make verify` or `make verify-extended`.

We are using Go modules for dependency management and [Ginkgo](https://github.com/onsi/ginkgo) / [Gomega](https://github.com/onsi/gomega) for unit tests.

To run integration tests against the locally running Azure controller:

- Execute `make pubip-remedy-test` to run integration tests for the [Cleanup orphaned public IP addresses](#cleanup-orphaned-public-ip-addresses) Azure remedy described above.

### Testing Locally

Testing specific remedies is tricky, since reproducing actual problematic situations that would require the remedy controller to do its job can be difficult or impossible. Below you can find instructions for simulating such problematic situations in order to test the remedy controllers.

#### Testing Azure Remedies

##### Cleanup orphaned public IP addresses

To test this remedy:

1. Create a new service of type `LoadBalancer` in the target cluster.

2. After a couple of minutes, make sure that a corresponding `PublicIPAddress` resource exists in the control cluster and its IP address matches the IP address in the service status.

3. Delete the service (or change its type to `ClusterIP`).

4. Make sure that after the public IP address has been deleted from Azure, the `PublicIPAddress` resource mentioned above is also deleted.

5. Create a new Azure public IP address manually. Optionally, you can also add it to the load balancer for the resource group by creating a frontend IP configuration, a load balancing rule, and a probe.

6. Create a `PublicIPAddress` resource in the control cluster for the newly created IP address, for example:

   ```yaml
   apiVersion: azure.remedy.gardener.cloud/v1alpha1
   kind: PublicIPAddress
   metadata:
     name: foo
   spec:
     ipAddress: 51.138.42.226
   ```

7. Delete the `PublicIPAddress` resource.

8. Make sure that after the configured deletion grace period has elapsed, the public IP address is deleted from Azure.

##### Reapply failed VMs

To test this remedy:

1. Add a new node to your cluster.

2. Make sure that a corresponding `VirtualMachine` resource exists in the control cluster.

3. Induce a `Failed` state for the VM of the new node by executing:

   ```bash
   make start-failedvm-simulator-azure VM_NAME=<node-name>
   ```

4. Monitor the `VirtualMachine` resource. In the status of the resource, you should see `failedOperations` indicating that a number of attempts to reapply the VM have been performed (and they all failed). This number of attempts should increase a few times.

5. After the configured maximum number of attempts for the reapply operation has been reached, you should not see any more changes to the `VirtualMachine` resource status.

## Feedback and Support

Feedback and contributions are always welcome. Please report bugs or suggestions as [GitHub issues](https://github.com/gardener/remedy-controller/issues) or join our [Slack channel #gardener](https://kubernetes.slack.com/messages/gardener) (please invite yourself to the Kubernetes workspace [here](http://slack.k8s.io)).

## Learn More!

You can find more information about Gardener here:

- [Our landing page gardener.cloud](https://gardener.cloud/)
- ["Gardener, the Kubernetes Botanist" blog on kubernetes.io](https://kubernetes.io/blog/2018/05/17/gardener/)
- [Gardener Extensions Golang library](https://godoc.org/github.com/gardener/gardener/extensions/pkg)
- [Gardener API Reference](https://gardener.cloud/api-reference/)
