FROM brew.registry.redhat.io/rh-osbs/openshift-golang-builder:rhel_9_1.24 as builder
WORKDIR /go/src/github.com/openshift/cli-manager-operator
COPY . .
RUN make build --warn-undefined-variables \
    && gzip cli-manager-operator-tests-ext

FROM registry.access.redhat.com/ubi9/ubi-minimal:latest@sha256:bb08f2300cb8d12a7eb91dddf28ea63692b3ec99e7f0fa71a1b300f2756ea829
COPY --from=builder /go/src/github.com/openshift/cli-manager-operator/cli-manager-operator /usr/bin/
COPY --from=builder /go/src/github.com/openshift/cli-manager-operator/cli-manager-operator-tests-ext.gz /usr/bin/
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
