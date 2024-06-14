FROM brew.registry.redhat.io/rh-osbs/openshift-golang-builder:rhel_9_1.22 as builder
WORKDIR /go/src/github.com/openshift/cli-manager-operator
COPY . .

ARG OPERATOR_IMAGE=registry.redhat.io/cli-manager-operator/cli-manager-rhel9-operator:latest
ARG OPERATOR_IMAGE_2=registry.redhat.io/cli-manager-operator/cli-manager-rhel9-operator@sha256:b0ca932fef93c81f5415aec4a4118492cf04bb3b8780f648c1287824adb8b7e7
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

COPY --from=builder manifests /manifests/
COPY --from=builder metadata /metadata/

LABEL io.k8s.display-name="CLI Manager Operator" \
      io.k8s.description="This is a component of OpenShift and manages the CLI Manager" \
      io.openshift.tags="openshift,cli-manager-operator" \
      com.redhat.delivery.appregistry=true \
      maintainer="AOS workloads team, <aos-workloads-staff@redhat.com>"
