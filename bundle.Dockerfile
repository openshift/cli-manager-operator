FROM brew.registry.redhat.io/rh-osbs/openshift-golang-builder:rhel_9_1.22 as builder
WORKDIR /go/src/github.com/openshift/cli-manager-operator
COPY . .

ARG OPERAND_IMAGE=registry.redhat.io/cli-manager/cli-manager-rhel9@sha256:0924ac7635541efb48832c5606d9a312bd7793216344b5e93921a56f421b40fe
ARG REPLACED_OPERAND_IMG=quay.io/openshift/origin-cli-manager:latest

# Replace the operand image in deploy/07_deployment.yaml with the one specified by the OPERAND_IMAGE build argument.
RUN hack/replace-image.sh deploy $REPLACED_OPERAND_IMG $OPERAND_IMAGE
RUN hack/replace-image.sh manifests $REPLACED_OPERAND_IMG $OPERAND_IMAGE

ARG OPERATOR_IMAGE=registry.redhat.io/cli-manager/cli-manager-rhel9-operator@sha256:74b5366689aba90d2ce315dd50995fb4d3c233ee0d0da2e57bd55333d1136312
ARG REPLACED_OPERATOR_IMG=quay.io/openshift/origin-cli-manager-operator:latest

# Replace the operand image in deploy/07_deployment.yaml with the one specified by the OPERATOR_IMAGE build argument.
RUN hack/replace-image.sh deploy $REPLACED_OPERATOR_IMG $OPERATOR_IMAGE
RUN hack/replace-image.sh manifests $REPLACED_OPERATOR_IMG $OPERATOR_IMAGE

RUN mkdir licenses
COPY LICENSE licenses/.

FROM registry.redhat.io/rhel9-4-els/rhel-minimal:9.4

LABEL operators.operatorframework.io.bundle.mediatype.v1=registry+v1
LABEL operators.operatorframework.io.bundle.manifests.v1=manifests/
LABEL operators.operatorframework.io.bundle.metadata.v1=metadata/
LABEL operators.operatorframework.io.bundle.package.v1=cli-manager
LABEL operators.operatorframework.io.bundle.channels.v1=tech-preview
LABEL operators.operatorframework.io.bundle.channel.default.v1=tech-preview
LABEL operators.operatorframework.io.metrics.builder=operator-sdk-v1.34.2
LABEL operators.operatorframework.io.metrics.mediatype.v1=metrics+v1
LABEL operators.operatorframework.io.metrics.project_layout=go.kubebuilder.io/v4

COPY --from=builder /go/src/github.com/openshift/cli-manager-operator/manifests /manifests
COPY --from=builder /go/src/github.com/openshift/cli-manager-operator/metadata /metadata
COPY --from=builder /go/src/github.com/openshift/cli-manager-operator/licenses /licenses

LABEL com.redhat.component="CLI Manager"
LABEL description="The CLI Manager is a comprehensive tool designed to simplify the management of OpenShift CLI plugins within the OpenShift environment. Modeled after the popular krew plugin manager, it offers seamless integration, easy installation, and efficient handling of a wide array of plugins, enhancing your OpenShift command-line experience."
LABEL distribution-scope="public"
LABEL name="cli-manager-operator-bundle"
LABEL release="0.1.0"
LABEL version="0.1.0"
LABEL url="https://github.com/openshift/cli-manager-operator"
LABEL vendor="Red Hat, Inc."
LABEL summary="The CLI Manager is a comprehensive tool designed to simplify the management of OpenShift CLI plugins within the OpenShift environment. Modeled after the popular krew plugin manager, it offers seamless integration, easy installation, and efficient handling of a wide array of plugins, enhancing your OpenShift command-line experience."

LABEL io.k8s.display-name="CLI Manager Operator Bundle" \
      io.k8s.description="This is a bundle image for CLI Manager" \
      io.openshift.tags="openshift,cli-manager-operator,cli-manager" \
      com.redhat.delivery.appregistry=true \
      maintainer="AOS workloads team, <aos-workloads-staff@redhat.com>"
USER 1001

