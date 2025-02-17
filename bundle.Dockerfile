FROM brew.registry.redhat.io/rh-osbs/openshift-golang-builder:rhel_9_1.22 as builder
WORKDIR /go/src/github.com/openshift/cli-manager-operator
COPY . .

# FIXME: update to registry.redhat.io, when this version is released to production
ARG OPERAND_IMAGE=registry.stage.redhat.io/cli-manager/cli-manager-rhel9@sha256:135dfe2ae5ff4a326a21a6039f1c4a8da8d90c924e1e383f88b34bbd261efa22
ARG REPLACED_OPERAND_IMG=quay.io/openshift/origin-cli-manager:latest

# Replace the operand image in deploy/07_deployment.yaml with the one specified by the OPERAND_IMAGE build argument.
RUN hack/replace-image.sh deploy $REPLACED_OPERAND_IMG $OPERAND_IMAGE
RUN hack/replace-image.sh manifests $REPLACED_OPERAND_IMG $OPERAND_IMAGE

# FIXME: update to registry.redhat.io, when this version is released to production
ARG OPERATOR_IMAGE=registry.stage.redhat.io/cli-manager/cli-manager-rhel9-operator@sha256:6c1e8ac7aff96cead1860af4a1233bac7086fb79d48cefefaeb2878d8c670b89
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
LABEL release="1.0.0"
LABEL version="1.0.0"
LABEL url="https://github.com/openshift/cli-manager-operator"
LABEL vendor="Red Hat, Inc."
LABEL summary="The CLI Manager is a comprehensive tool designed to simplify the management of OpenShift CLI plugins within the OpenShift environment. Modeled after the popular krew plugin manager, it offers seamless integration, easy installation, and efficient handling of a wide array of plugins, enhancing your OpenShift command-line experience."

LABEL io.k8s.display-name="CLI Manager Operator Bundle" \
      io.k8s.description="This is a bundle image for CLI Manager" \
      io.openshift.tags="openshift,cli-manager-operator,cli-manager" \
      com.redhat.delivery.appregistry=true \
      maintainer="AOS workloads team, <aos-workloads-staff@redhat.com>"
USER 1001

