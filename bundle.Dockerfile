FROM brew.registry.redhat.io/rh-osbs/openshift-golang-builder:rhel_9_1.22 as builder
WORKDIR /go/src/github.com/openshift/cli-manager-operator
COPY . .

ARG OPERAND_IMAGE=registry.redhat.io/cli-manager-operator/cli-manager-rhel9@sha256:864d85b8509c3888c09ed03ce42380d5708f096901726f559c47e96549e0e375
ARG REPLACED_OPERAND_IMG=quay.io/openshift/origin-cli-manager:latest

# Replace the operand image in deploy/07_deployment.yaml with the one specified by the OPERAND_IMAGE build argument.
RUN hack/replace-image.sh deploy $REPLACED_OPERAND_IMG $OPERAND_IMAGE
RUN hack/replace-image.sh manifests $REPLACED_OPERAND_IMG $OPERAND_IMAGE

ARG OPERATOR_IMAGE=registry.redhat.io/cli-manager-operator/cli-manager-rhel9-operator@sha256:e7069cc033345709477cf06a96b4c884d49b8de28b5ca37a04ea4a74ba2917de
ARG REPLACED_OPERATOR_IMG=quay.io/openshift/origin-cli-manager-operator:latest

# Replace the operand image in deploy/07_deployment.yaml with the one specified by the OPERATOR_IMAGE build argument.
RUN hack/replace-image.sh deploy $REPLACED_OPERATOR_IMG $OPERATOR_IMAGE
RUN hack/replace-image.sh manifests $REPLACED_OPERATOR_IMG $OPERATOR_IMAGE

FROM scratch

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

LABEL io.k8s.display-name="CLI Manager Operator" \
      io.k8s.description="This is a component of OpenShift and manages the CLI Manager" \
      io.openshift.tags="openshift,cli-manager-operator" \
      com.redhat.delivery.appregistry=true \
      maintainer="AOS workloads team, <aos-workloads-staff@redhat.com>"
