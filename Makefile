all: build
.PHONY: all

SOURCE_GIT_TAG ?=$(shell git describe --long --tags --abbrev=7 --match 'v[0-9]*' || echo 'v1.0.0-$(SOURCE_GIT_COMMIT)')
SOURCE_GIT_COMMIT ?=$(shell git rev-parse --short "HEAD^{commit}" 2>/dev/null)

# OS_GIT_VERSION is populated by ART
# If building out of the ART pipeline, fallback to SOURCE_GIT_TAG
ifndef OS_GIT_VERSION
	OS_GIT_VERSION = $(SOURCE_GIT_TAG)
endif

# Include the library makefile
include $(addprefix ./vendor/github.com/openshift/build-machinery-go/make/, \
	golang.mk \
	targets/openshift/images.mk \
	targets/openshift/codegen.mk \
	targets/openshift/deps.mk \
	targets/openshift/crd-schema-gen.mk \
)

# Exclude e2e tests from unit testing
GO_TEST_PACKAGES :=./pkg/... ./cmd/...
GO_BUILD_FLAGS :=-tags strictfipsruntime

IMAGE_REGISTRY :=registry.svc.ci.openshift.org

CODEGEN_OUTPUT_PACKAGE :=github.com/openshift/cli-manager-operator/pkg/generated
CODEGEN_API_PACKAGE :=github.com/openshift/cli-manager-operator/pkg/apis
CODEGEN_GROUPS_VERSION :=climanager:v1

# This will call a macro called "build-image" which will generate image specific targets based on the parameters:
# $0 - macro name
# $1 - target name
# $2 - image ref
# $3 - Dockerfile path
# $4 - context directory for image build
$(call build-image,ocp-cli-manager-operator,$(IMAGE_REGISTRY)/ocp/5.1:cli-manager-operator, ./Dockerfile,.)

$(call verify-golang-versions,Dockerfile)

# Regenerate the CRD with controller-gen built from vendored controller-tools.
.PHONY: regen-crd
regen-crd:
	go build -o _output/tools/bin/controller-gen ./vendor/sigs.k8s.io/controller-tools/cmd/controller-gen
	cp manifests/openshift-cli-manager-operator.crd.yaml manifests/operator.openshift.io_climanagers.yaml
	_output/tools/bin/controller-gen crd:crdVersions=v1 paths=./pkg/apis/climanager/v1 output:crd:dir=./manifests
	mv manifests/operator.openshift.io_climanagers.yaml manifests/openshift-cli-manager-operator.crd.yaml
	sed -i '1{/^---$$/d;}' manifests/openshift-cli-manager-operator.crd.yaml
	cp manifests/openshift-cli-manager-operator.crd.yaml deploy/00_cli-manager-operator.crd.yaml
	cp manifests/openshift-cli-manager-operator.crd.yaml test/e2e/bindata/assets/00_cli-manager-operator.crd.yaml

update-codegen-crds: regen-crd
.PHONY: update-codegen-crds

verify-codegen-crds: update-codegen-crds
	git diff --exit-code
.PHONY: verify-codegen-crds

test-e2e: GO_TEST_PACKAGES :=./test/e2e
# the e2e imports pkg/cmd which has a data race in the transport library with the library-go init code
test-e2e: GO_TEST_FLAGS :=-v -timeout=3h
test-e2e: test-unit
.PHONY: test-e2e

generate: update-codegen-crds generate-clients
.PHONY: generate

generate-clients:
	GO=GO111MODULE=on GOFLAGS=-mod=readonly hack/update-codegen.sh
.PHONY: generate-clients

clean:
	$(RM) ./cli-manager-operator
.PHONY: clean