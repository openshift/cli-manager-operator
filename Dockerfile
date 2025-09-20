FROM brew.registry.redhat.io/rh-osbs/openshift-golang-builder:rhel_9_1.22 as builder
WORKDIR /go/src/github.com/openshift/cli-manager-operator
COPY . .
RUN make build --warn-undefined-variables

FROM registry.redhat.io/rhel9-4-els/rhel:9.4
COPY --from=builder /go/src/github.com/openshift/cli-manager-operator/cli-manager-operator /usr/bin/
RUN mkdir /licenses
COPY --from=builder /go/src/github.com/openshift/cli-manager-operator/LICENSE /licenses/.

LABEL com.redhat.component="CLI Manager"
LABEL description="The CLI Manager is a comprehensive tool designed to simplify the management of OpenShift CLI plugins within the OpenShift environment. Modeled after the popular krew plugin manager, it offers seamless integration, easy installation, and efficient handling of a wide array of plugins, enhancing your OpenShift command-line experience."
LABEL name="cli-manager/cli-manager-rhel9-operator"
LABEL cpe="cpe:/a:redhat:cli_manager_operator:0.1::el9"
LABEL summary="The CLI Manager is a comprehensive tool designed to simplify the management of OpenShift CLI plugins within the OpenShift environment. Modeled after the popular krew plugin manager, it offers seamless integration, easy installation, and efficient handling of a wide array of plugins, enhancing your OpenShift command-line experience."
LABEL io.k8s.display-name="CLI Manager Operator" \
      io.k8s.description="This is an operator to manage CLI Manager" \
      io.openshift.tags="openshift,cli-manager-operator" \
      com.redhat.delivery.appregistry=true \
      maintainer="AOS workloads team, <aos-workloads-staff@redhat.com>"
USER 1001
