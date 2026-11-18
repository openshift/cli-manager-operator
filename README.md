# CLI Manager Operator

Run the CLI Manager in your OpenShift cluster to distribute CLIs.

## Releases

| version | ocp version | k8s version | golang |
|---------|-------------|-------------|--------|
| 0.1.0   | 4.17-4.21   | 1.30        | 1.22   |
| 0.1.1   | 4.18-4.22   | 1.32        | 1.23   |
| 0.2.0   | 4.19-4.23, 5.0   | 1.36        | 1.26   |
| 0.3.0 (upcoming)   | 5.1         | 1.37        | 1.26   |

## Deploy the operator

### Quick Development

1. Build and push the operator image to a registry:
2. Ensure the `image` spec in `deploy/09_deployment.yaml` refers to the operator image you pushed
3. Run `oc create -f deploy/.`

### OperatorHub install with custom index image

This process is used for building index images from bundle images for local OperatorHub installation.

**Prerequisites**: Log in to registry.redhat.io before building, as the Dockerfile and bundle.Dockerfile pull base images from there:
   ```sh
   podman login registry.redhat.io
   ```

1. Build and push the operator image to a registry:
   ```sh
   export QUAY_USER=${your_quay_user_id}
   export IMAGE_TAG=${your_image_tag}
   podman build -t quay.io/${QUAY_USER}/openshift-cli-manager-operator:${IMAGE_TAG} -f Dockerfile .
   podman login quay.io -u ${QUAY_USER}
   podman push quay.io/${QUAY_USER}/openshift-cli-manager-operator:${IMAGE_TAG}
   ```

1. Build and push the bundle image:
   ```sh
   # Build bundle image. Optionally specify --build-arg OPERATOR_IMAGE=<image> to reference
   # a different operator image in the bundle; omitting it preserves the default.
   podman build -t quay.io/${QUAY_USER}/openshift-cli-manager-operator-bundle:${IMAGE_TAG} -f bundle.Dockerfile .
   podman push quay.io/${QUAY_USER}/openshift-cli-manager-operator-bundle:${IMAGE_TAG}
   ```

1. Build and push the index image using `opm` (install from https://github.com/operator-framework/operator-registry/):
   ```sh
   opm index add --bundles quay.io/${QUAY_USER}/openshift-cli-manager-operator-bundle:${IMAGE_TAG} \
     --tag quay.io/${QUAY_USER}/openshift-cli-manager-operator-index:${IMAGE_TAG}
   podman push quay.io/${QUAY_USER}/openshift-cli-manager-operator-index:${IMAGE_TAG}
   ```

   Don't forget to increase the number of open files, e.g. `ulimit -n 100000` in case the current limit is insufficient.

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

See [CONTRIBUTING.md](CONTRIBUTING.md) for testing guidelines ([unit/e2e](CONTRIBUTING.md#testing-guidelines), [OTE](CONTRIBUTING.md#openshift-tests-extension-ote)).