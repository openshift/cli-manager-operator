FROM brew.registry.redhat.io/rh-osbs/openshift-golang-builder:rhel_9_1.22 as builder
WORKDIR /go/src/github.com/openshift/cli-manager-operator
COPY . .

ARG OPERAND_IMAGE=registry.redhat.io/cli-manager-operator/cli-manager-rhel9@sha256:7a359fa2019251ebffcc2f5a03324296968e0dc3a578f6810479f9c88125e7b5
ARG REPLACED_OPERAND_IMG=quay.io/openshift/origin-cli-manager:latest

# Replace the operand image in deploy/07_deployment.yaml with the one specified by the OPERAND_IMAGE build argument.
RUN hack/replace-image.sh deploy $REPLACED_OPERAND_IMG $OPERAND_IMAGE
RUN hack/replace-image.sh manifests $REPLACED_OPERAND_IMG $OPERAND_IMAGE

ARG OPERATOR_IMAGE=registry.redhat.io/cli-manager-operator/cli-manager-rhel9-operator@sha256:eae66ed5a6576d58642db6a2908c609d64745091b21448caee01db9e2d691838
ARG REPLACED_OPERATOR_IMG=quay.io/openshift/origin-cli-manager-operator:latest

# Replace the operand image in deploy/07_deployment.yaml with the one specified by the OPERATOR_IMAGE build argument.
RUN hack/replace-image.sh deploy $REPLACED_OPERATOR_IMG $OPERATOR_IMAGE
RUN hack/replace-image.sh manifests $REPLACED_OPERATOR_IMG $OPERATOR_IMAGE

RUN mkdir licenses
COPY LICENSE licenses/.

FROM registry.redhat.io/rhel9-4-els/rhel:9.4

LABEL operators.operatorframework.io.bundle.mediatype.v1=registry+v1
LABEL operators.operatorframework.io.bundle.manifests.v1=manifests/
LABEL operators.operatorframework.io.bundle.metadata.v1=metadata/
LABEL operators.operatorframework.io.bundle.package.v1=cli-manager-operator
LABEL operators.operatorframework.io.bundle.channels.v1=alpha
LABEL operators.operatorframework.io.bundle.channel.default.v1=preview
LABEL operators.operatorframework.io.metrics.builder=operator-sdk-v1.34.2
LABEL operators.operatorframework.io.metrics.mediatype.v1=metrics+v1
LABEL operators.operatorframework.io.metrics.project_layout=go.kubebuilder.io/v4

COPY --from=builder /go/src/github.com/openshift/cli-manager-operator/manifests /manifests
COPY --from=builder /go/src/github.com/openshift/cli-manager-operator/metadata /metadata
COPY --from=builder /go/src/github.com/openshift/cli-manager-operator/licenses /licenses

LABEL com.redhat.component="CLI Manager Operator"
LABEL description="The CLI Manager Operator delivers plugins as Krew compatible binaries"
LABEL distribution-scope="public"
LABEL name="cli-manager-operator-bundle"
LABEL release="0.1"
LABEL version="0.1"
LABEL url="https://github.com/openshift/cli-manager-operator"
LABEL vendor="Red Hat, Inc."
LABEL summary="This is a component of OpenShift and manages the CLI Manager"

LABEL io.k8s.display-name="CLI Manager Operator" \
      io.k8s.description="This is a component of OpenShift and manages the CLI Manager" \
      io.openshift.tags="openshift,cli-manager-operator,cli-manager" \
      com.redhat.delivery.appregistry=true \
      maintainer="AOS workloads team, <aos-workloads-staff@redhat.com>"
USER 1001

