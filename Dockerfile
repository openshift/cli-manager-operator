FROM brew.registry.redhat.io/rh-osbs/openshift-golang-builder:rhel_9_1.24 as builder
WORKDIR /go/src/github.com/openshift/cli-manager-operator
COPY . .
RUN make build --warn-undefined-variables

FROM registry.access.redhat.com/ubi9/ubi-minimal:latest@sha256:161a4e29ea482bab6048c2b36031b4f302ae81e4ff18b83e61785f40dc576f5d
COPY --from=builder /go/src/github.com/openshift/cli-manager-operator/cli-manager-operator /usr/bin/
RUN mkdir /licenses
COPY --from=builder /go/src/github.com/openshift/cli-manager-operator/LICENSE /licenses/.

LABEL com.redhat.component="CLI Manager"
LABEL description="The CLI Manager is a comprehensive tool designed to simplify the management of OpenShift CLI plugins within the OpenShift environment. Modeled after the popular krew plugin manager, it offers seamless integration, easy installation, and efficient handling of a wide array of plugins, enhancing your OpenShift command-line experience."
LABEL name="cli-manager-operator"
LABEL summary="The CLI Manager is a comprehensive tool designed to simplify the management of OpenShift CLI plugins within the OpenShift environment. Modeled after the popular krew plugin manager, it offers seamless integration, easy installation, and efficient handling of a wide array of plugins, enhancing your OpenShift command-line experience."
LABEL cpe="cpe:/a:redhat:cli_manager_operator:0.2::el9"
LABEL release="0.2.0"
LABEL version="0.2.0"
LABEL url="https://github.com/openshift/cli-manager-operator"
LABEL vendor="Red Hat, Inc."
LABEL io.k8s.display-name="CLI Manager Operator" \
      io.k8s.description="This is an operator to manage CLI Manager" \
      io.openshift.tags="openshift,cli-manager-operator,cli" \
      com.redhat.delivery.appregistry=true \
      distribution-scope=public \
      maintainer="AOS workloads team, <aos-workloads-staff@redhat.com>"
USER 1001
