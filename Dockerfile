FROM brew.registry.redhat.io/rh-osbs/openshift-golang-builder:rhel_9_1.22 as builder
WORKDIR /go/src/github.com/openshift/cli-manager-operator
COPY . .

ARG OPERAND_IMAGE=quay.io/redhat-user-workloads/clio-wrklds-pipeline-tenant/clio-wrklds-pipeline/cli-manager-operator@sha256:958d000fa2a270bc09cce7150530fb78d69c74e42b0f35b22422e319fbcf871b
ARG OPERAND_IMAGE_2=registry.redhat.io/clio-wrklds-pipeline-tenant/clio-wrklds-pipeline@sha256:958d000fa2a270bc09cce7150530fb78d69c74e42b0f35b22422e319fbcf871b
ARG OPERAND_IMAGE_3=registry.redhat.io/clio-wrklds-pipeline/clio-wrklds-pipeline@sha256:958d000fa2a270bc09cce7150530fb78d69c74e42b0f35b22422e319fbcf871b
ARG OPERAND_IMAGE_4=quay.io/redhat-services-prod/clio-wrklds-pipeline-cli-manager@sha256:b0ca932fef93c81f5415aec4a4118492cf04bb3b8780f648c1287824adb8b7e7
ARG REPLACED_OPERAND_IMG=quay.io/openshift/origin-cli-manager:latest

# Replace the operand image in deploy/07_deployment.yaml with the one specified by the OPERAND_IMAGE build argument.
RUN find deploy/ && find deploy -type f -exec sed -i \
    "s|${REPLACED_OPERAND_IMG}|${OPERAND_IMAGE_4}|g" {} \+; \
    grep -rq "${REPLACED_OPERAND_IMG}" deploy/ && \
    { echo "Failed to replace image references"; exit 1; } || echo "Image references replaced" && \
    grep -r "${OPERAND_IMAGE_4}" deploy/
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
