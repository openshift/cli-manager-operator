all: build
.PHONY: all

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
$(call build-image,ocp-cli-manager-operator,$(IMAGE_REGISTRY)/ocp/4.16:cli-manager-operator, ./Dockerfile,.)

$(call verify-golang-versions,Dockerfile)

$(call add-crd-gen,climanager,./pkg/apis/climanager/v1,./manifests/,./manifests/)

test-e2e: GO_TEST_PACKAGES :=./test/e2e
test-e2e: test-unit
.PHONY: test-e2e

generate: update-codegen-crds generate-clients
.PHONY: generate

generate-clients:
	bash ./vendor/k8s.io/code-generator/kube_codegen.sh	"applyconfiguration,client,deepcopy,informer,lister" github.com/openshift/cli-manager-operator/pkg/generated github.com/openshift/cli-manager-operator/pkg/apis climanager:v1 --go-header-file=./hack/boilerplate.go.txt
.PHONY: generate-clients

clean:
	$(RM) ./openshift-cli-manager-operator
.PHONY: clean