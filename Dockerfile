FROM brew.registry.redhat.io/rh-osbs/openshift-golang-builder:rhel_9_1.22 as builder
WORKDIR /go/src/github.com/openshift/cli-manager-operator
COPY . .

ARG OPERAND_IMAGE=registry.redhat.io/cli-manager-operator/cli-manager-rhel9@sha256:ec03071a0d4c83260b9c71a58bedfe006c18c9c1fb2b9b8f9ce4389da7f39e39
ARG REPLACED_OPERAND_IMG=quay.io/openshift/origin-cli-manager:latest

# Replace the operand image in deploy/07_deployment.yaml with the one specified by the OPERAND_IMAGE build argument.
RUN hack/replace-image.sh deploy $REPLACED_OPERAND_IMG $OPERAND_IMAGE
RUN hack/replace-image.sh manifests $REPLACED_OPERAND_IMG $OPERAND_IMAGE
RUN make build --warn-undefined-variables

FROM registry.redhat.io/rhel9-2-els/rhel:9.2-1260.1718293448
COPY --from=builder /go/src/github.com/openshift/cli-manager-operator/cli-manager-operator /usr/bin/
COPY --from=builder /go/src/github.com/openshift/cli-manager-operator/manifests /manifests
RUN mkdir /licenses
COPY --from=builder /go/src/github.com/openshift/cli-manager-operator/LICENSE /licenses/.
LABEL io.k8s.display-name="CLI Manager Operator" \
      io.k8s.description="This is a component of OpenShift and manages the CLI Manager" \
      io.openshift.tags="openshift,cli-manager-operator" \
      com.redhat.delivery.appregistry=true \
      maintainer="AOS workloads team, <aos-workloads-staff@redhat.com>"
USER 1001
