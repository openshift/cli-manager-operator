# CLI Manager Operator

Run the CLI Manager in your OpenShift cluster to distribute CLIs.

## Releases

| lws version | ocp version | k8s version | golang |
|-------------|-------------|-------------|--------|
| 0.1.0       | 4.17-4.21   | 1.30        | 1.22   |
| 0.1.1       | 4.18-4.21   | 1.34        | 1.24   |

## Deploy the operator

### Quick Development

1. Build and push the operator image to a registry:
2. Ensure the `image` spec in `deploy/07_deployment.yaml` refers to the operator image you pushed
3. Run `oc create -f deploy/.`

### OperatorHub install with custom index image

This process refers to building the operator in a way that it can be installed locally via the OperatorHub with a custom index image

1. Build and push the operator image to a registry:
   ```sh
   export QUAY_USER=${your_quay_user_id}
   export IMAGE_TAG=${your_image_tag}
   podman build -t quay.io/${QUAY_USER}/openshift-cli-manager-operator:${IMAGE_TAG} -f Dockerfile
   podman login quay.io -u ${QUAY_USER}
   podman push quay.io/${QUAY_USER}/openshift-cli-manager-operator:${IMAGE_TAG}
   ```

1. Export your desired/current version:

   ```sh
   export OPERATOR_VERSION=${your_version}
   ```

1. Update the `.spec.install.spec.deployments[0].spec.template.spec.containers[0].image` field in the SSO CSV under `./manifests/${OPERATOR_VERSION}/openshift-cli-manager-operator.v${OPERATOR_VERSION}.0.clusterserviceversion.yaml` to point to the newly built image.

1. build and push the metadata image to a registry (e.g. https://quay.io):
   ```sh
   podman build -t quay.io/${QUAY_USER}/openshift-cli-manager-operator-metadata:${IMAGE_TAG} -f Dockerfile.metadata .
   podman push quay.io/${QUAY_USER}/openshift-cli-manager-metadata:${IMAGE_TAG}
   ```

1. build and push image index for operator-registry (pull and build https://github.com/operator-framework/operator-registry/ to get the `opm` binary)
   ```sh
   opm index add --bundles quay.io/${QUAY_USER}/openshift-cli-manager-operator-metadata:${IMAGE_TAG} --tag quay.io/${QUAY_USER}/openshift-cli-manager-operator-index:${IMAGE_TAG}
   podman push quay.io/${QUAY_USER}/openshift-cli-manager-operator-index:${IMAGE_TAG}
   ```

   Don't forget to increase the number of open files, .e.g. `ulimit -n 100000` in case the current limit is insufficient.

1. create and apply catalogsource manifest (remember to change <<QUAY_USER>> and <<IMAGE_TAG>> to your own values):
   ```yaml
   apiVersion: operators.coreos.com/v1alpha1
   kind: CatalogSource
   metadata:
     name: openshift-cli-manager-operator
     namespace: openshift-marketplace
   spec:
     sourceType: grpc
     image: quay.io/<<QUAY_USER>>/openshift-cli-manager-operator-index:<<IMAGE_TAG>>
   ```

1. create `openshift-cli-manager-operator` namespace:
   ```
   $ oc create ns openshift-cli-manager-operator
   ```

1. open the console Operators -> OperatorHub, search for `CLI Manager operator` and install the operator

## Tests

This repository is compatible with the [OpenShift Tests Extension (OTE)](https://github.com/openshift-eng/openshift-tests-extension) framework.

### Building the test binary

```bash
make build
```

### Running test suites and tests

```bash
# Run a specific test suite or test
./cli-manager-operator-tests-ext run-suite openshift/cli-manager-operator/operator/serial
./cli-manager-operator-tests-ext run-test "test-name"

# To run serial suites cases serially, use the following command:
./cli-manager-operator-tests-ext run-suite openshift/cli-manager-operator/operator/serial -c 1

# Run with JUnit output
./cli-manager-operator-tests-ext run-suite openshift/cli-manager-operator/operator/serial --junit-path=/tmp/junit.xml
./cli-manager-operator-tests-ext run-test "test-name" --junit-path=/tmp/junit.xml
```

### Listing available tests and suites

```bash
# List all test suites
./cli-manager-operator-tests-ext list suites

# List tests in a suite
./cli-manager-operator-tests-ext list tests --suite=openshift/cli-manager-operator/operator/serial
```

For more information about the OTE framework, see the [openshift-tests-extension documentation](https://github.com/openshift-eng/openshift-tests-extension).