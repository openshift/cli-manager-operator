FROM brew.registry.redhat.io/rh-osbs/openshift-golang-builder:rhel_9_1.21 as builder
WORKDIR /go/src/github.com/openshift/cli-manager-operator
COPY . .
RUN make build --warn-undefined-variables

FROM registry.redhat.io/rhel9-2-els/rhel:9.2-1222
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
