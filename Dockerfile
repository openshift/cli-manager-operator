FROM brew.registry.redhat.io/rh-osbs/openshift-golang-builder:rhel_9_1.21 as builder
WORKDIR /go/src/github.com/openshift/cli-manager-operator
COPY . .
RUN make build --warn-undefined-variables

FROM registry.redhat.io/rhel9-2-els/rhel:9.2-1222
ENV OPERAND_IMAGE=quay.io/redhat-user-workloads/clio-wrklds-pipeline-tenant/clio-wrklds-pipeline/cli-manager-operator@sha256:9fec14cdb694beba7afc34198ccfd54ccdeaefe634ccff466cdde04d7ddfbe6d
ENV OPERAND_IMAGE_2=registry.redhat.io/clio-wrklds-pipeline-tenant/clio-wrklds-pipeline@sha256:9fec14cdb694beba7afc34198ccfd54ccdeaefe634ccff466cdde04d7ddfbe6d
ENV OPERAND_IMAGE_3=registry.redhat.io/clio-wrklds-pipeline/clio-wrklds-pipeline@sha256:9fec14cdb694beba7afc34198ccfd54ccdeaefe634ccff466cdde04d7ddfbe6d
ENV OPERAND_IMAGE_4=quay.io/redhat-services-prod/clio-wrklds-pipeline-cli-manager@sha256:39143e4fb165a99d9a3bfee6ec6f6828a54538e06b31ee356923fb95c32ab233
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
