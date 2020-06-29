## Running the integration test locally

### Credentials

Before running the test you should check whether you have the necessary files prepared. The test currently expects two things:

1. A `credentials.yaml` that contains the necessary credentials (and as of now: region information) for interacting with the shoots' resources on azure. The file is expected to have the following structure:
    ```yaml
    aadClientId: "<client id>"
    aadClientSecret: "<client secret>"
    tenantId: "<tenant id>"
    subscriptionId: "<subscription id>"
    resourceGroup: "<resource group name>"
    location: "<azure region name>"
    ```
2. A `kubeconfig` that points to the shoot cluster.

### Using Make

Ensure that the virtual environment is created using
```
make install-requirements
```
then, run the test:
```
make pubip-remedy-test
```

### Manually running the test with Python3

The non-standard Python3 packages the testing script depends on are included in the `requirements.txt` in this folder. It is generally recommended to create a virtual environment when setting up a new project (e.g. in the `.env` folder in the repository root) in order avoid having to install dependencies globally. This can be done by executing
```
python3 -m venv <repo root>/.env
```
Afterwards, the new virtual environment can be activated using
```
source <repo root>/.env/bin/activate
```
Finally, the dependencies can now be installed using
```
pip3 install -r test/requirements.txt
```

With everything set up, running the test is as simple as
```
python3 test/pubip-remedy_test.py <args>
```

To show the help message, detailing supported args and their defaults, use
```
python3 test/pubip_remedy_test.py --help
```
