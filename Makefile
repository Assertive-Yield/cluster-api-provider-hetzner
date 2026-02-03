# Copyright 2022 The Kubernetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

INFRA_SHORT = caph
IMAGE_PREFIX ?= ghcr.io/assertive-yield
INFRA_PROVIDER = hetzner

STAGING_IMAGE = $(INFRA_SHORT)-staging
BUILDER_IMAGE = $(IMAGE_PREFIX)/$(INFRA_SHORT)-builder
BUILDER_IMAGE_VERSION = $(shell cat .builder-image-version.txt)

SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec
.DEFAULT_GOAL:=help
##@ General


# The help target prints out all targets with their descriptions organized
# beneath their categories. The categories are represented by '##@' and the
# target descriptions by '##'. The awk commands is responsible for reading the
# entire set of makefiles included in this invocation, looking for lines of the
# file as xyz: ## something, and then pretty-format the target and help. Then,
# if there's a line with ##@ something, that gets pretty-printed as a category.
# More info on the usage of ANSI control characters for terminal formatting:
# https://en.wikipedia.org/wiki/ANSI_escape_code#SGR_parameters
# More info on the awk command:
# http://linuxcommand.org/lc3_adv_awk.php

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

#############
# Variables #
#############

# Certain aspects of the build are done in containers for consistency (e.g. protobuf generation)
# If you have the correct tools installed and you want to speed up development you can run
# make BUILD_IN_CONTAINER=false target
# or you can override this with an environment variable
BUILD_IN_CONTAINER ?= true

# Boiler plate for building Docker containers.
ARCH ?= amd64
# Allow overriding the imagePullPolicy
PULL_POLICY ?= Always
# Build time versioning details.
LDFLAGS := $(shell hack/version.sh)

# Directories
ROOT_DIR:=$(shell dirname $(realpath $(firstword $(MAKEFILE_LIST))))
BIN_DIR := bin
TOOLS_DIR := hack/tools
TOOLS_BIN_DIR := $(TOOLS_DIR)/$(BIN_DIR)
export PATH := $(abspath $(TOOLS_BIN_DIR)):$(PATH)
export GOBIN := $(abspath $(TOOLS_BIN_DIR))

# Kubebuilder.
# go install sigs.k8s.io/controller-runtime/tools/setup-envtest@latest
# The command `setup-envtest list` shows the available versions.
export KUBEBUILDER_ENVTEST_KUBERNETES_VERSION ?= 1.31.0

##@ Binaries
############
# Binaries #
############
CONTROLLER_GEN := $(abspath $(TOOLS_BIN_DIR)/controller-gen)
controller-gen: $(CONTROLLER_GEN) ## Build a local copy of controller-gen
$(CONTROLLER_GEN): # Build controller-gen from tools folder.
	go install sigs.k8s.io/controller-tools/cmd/controller-gen@v0.19.0

KUSTOMIZE := $(abspath $(TOOLS_BIN_DIR)/kustomize)
kustomize: $(KUSTOMIZE) ## Build a local copy of kustomize
$(KUSTOMIZE): # Build kustomize from tools folder.
	go install sigs.k8s.io/kustomize/kustomize/v4@v4.5.7

ENVSUBST := $(abspath $(TOOLS_BIN_DIR)/envsubst)
envsubst: $(ENVSUBST) ## Build a local copy of envsubst
$(ENVSUBST): # Build envsubst from tools folder.
	go install github.com/drone/envsubst/v2/cmd/envsubst@latest

SETUP_ENVTEST := $(abspath $(TOOLS_BIN_DIR)/setup-envtest)
setup-envtest: $(SETUP_ENVTEST) ## Build a local copy of setup-envtest
$(SETUP_ENVTEST): # Build setup-envtest from tools folder.
	go install sigs.k8s.io/controller-runtime/tools/setup-envtest@v0.0.0-20250310021545-f80bc5dbf8f7

CLUSTERCTL := $(abspath $(TOOLS_BIN_DIR)/clusterctl)
clusterctl: $(CLUSTERCTL) ## Build a local copy of clusterctl
$(CLUSTERCTL):
	go install sigs.k8s.io/cluster-api/cmd/clusterctl@v1.8.10

HELM := $(abspath $(TOOLS_BIN_DIR)/helm)
helm: $(HELM) ## Build a local copy of helm
$(HELM):
	curl -sSL https://get.helm.sh/helm-v3.13.2-$$(go env GOOS)-$$(go env GOARCH).tar.gz | tar xz -C $(TOOLS_BIN_DIR) --strip-components=1 $$(go env GOOS)-$$(go env GOARCH)/helm
	chmod a+rx $(HELM)

HCLOUD := $(abspath $(TOOLS_BIN_DIR)/hcloud)
hcloud: $(HCLOUD) ## Build a local copy of hcloud
$(HCLOUD):
	curl -sSL https://github.com/hetznercloud/cli/releases/download/v1.43.1/hcloud-$$(go env GOOS)-$$(go env GOARCH).tar.gz | tar xz -C $(TOOLS_BIN_DIR) hcloud
	chmod a+rx $(HCLOUD)

KUBECTL := $(abspath $(TOOLS_BIN_DIR)/kubectl)
kubectl: $(KUBECTL) ## Build a local copy of kubectl
$(KUBECTL):
	curl -fsSL "https://dl.k8s.io/release/v1.31.6/bin/$$(go env GOOS)/$$(go env GOARCH)/kubectl" -o $(KUBECTL)
	chmod a+rx $(KUBECTL)

GOTESTSUM := $(abspath $(TOOLS_BIN_DIR)/gotestsum)
gotestsum: $(GOTESTSUM) # Build gotestsum from tools folder.
$(GOTESTSUM):
	go install gotest.tools/gotestsum@v1.11.0

all-tools: $(GOTESTSUM) $(KUBECTL) $(CLUSTERCTL) $(SETUP_ENVTEST) $(ENVSUBST) $(KUSTOMIZE) $(CONTROLLER_GEN) $(HELM) ## Install all tools required for development
	echo 'done'

##@ Releasing
#############
# Releasing #
#############
## latest git tag for the commit, e.g., v0.3.10
RELEASE_TAG ?= $(shell git describe --abbrev=0 2>/dev/null)
# the previous release tag, e.g., v0.3.9, excluding pre-release tags
PREVIOUS_TAG ?= $(shell git tag -l | grep -E "^v[0-9]+\.[0-9]+\.[0-9]." | sort -V | grep -B1 $(RELEASE_TAG) | head -n 1 2>/dev/null)
RELEASE_DIR ?= out
RELEASE_NOTES_DIR := _releasenotes

$(RELEASE_DIR):
	mkdir -p $(RELEASE_DIR)/

$(RELEASE_NOTES_DIR):
	mkdir -p $(RELEASE_NOTES_DIR)/

.PHONY: test-release
test-release:
	@# TAG: caph container image tag. For PRs this is pr-NNNN
	./hack/ensure-env-variables.sh TAG
	$(MAKE) set-manifest-image MANIFEST_IMG=$(IMAGE_PREFIX)/$(STAGING_IMAGE) MANIFEST_TAG=$(TAG)
	$(MAKE) set-manifest-pull-policy PULL_POLICY=IfNotPresent
	$(MAKE) release-manifests

.PHONY: release-manifests
release-manifests: generate-manifests generate-go-deepcopy $(KUSTOMIZE) $(RELEASE_DIR) cluster-templates ## Builds the manifests to publish with a release
	$(KUSTOMIZE) build config/default > $(RELEASE_DIR)/infrastructure-components.yaml
	## Build $(INFRA_SHORT)-components (aggregate of all of the above).
	cp metadata.yaml $(RELEASE_DIR)/metadata.yaml
	cp templates/cluster-templates/cluster-template* $(RELEASE_DIR)/
	cp templates/cluster-templates/cluster-class* $(RELEASE_DIR)/

.PHONY: release
release: clean-release  ## Builds and push container images using the latest git tag for the commit.
	@if [ -z "${RELEASE_TAG}" ]; then echo "RELEASE_TAG is not set"; exit 1; fi
	@if ! [ -z "$$(git status --porcelain)" ]; then echo "Your local git repository contains uncommitted changes, use git clean before proceeding."; exit 1; fi
	git checkout "${RELEASE_TAG}"
	# Set the manifest image to the production bucket.
	$(MAKE) set-manifest-image MANIFEST_IMG=$(IMAGE_PREFIX)/$(INFRA_SHORT) MANIFEST_TAG=$(RELEASE_TAG)
	$(MAKE) set-manifest-pull-policy PULL_POLICY=IfNotPresent
	## Build the manifests
	$(MAKE) release-manifests clean-release-git
	./hack/check-release-manifests.sh


.PHONY: release-notes
release-notes: $(RELEASE_NOTES_DIR) $(RELEASE_NOTES)
	go run ./hack/tools/release/notes.go --from=$(PREVIOUS_TAG) > $(RELEASE_NOTES_DIR)/$(RELEASE_TAG).md

##@ Images
##########
# Images #
##########

.PHONY: set-manifest-image
set-manifest-image:
	$(info Updating kustomize image patch file for default resource)
	sed -i'' -e 's@image: .*@image: '"${MANIFEST_IMG}:$(MANIFEST_TAG)"'@' ./config/default/manager_config_patch.yaml

.PHONY: set-manifest-pull-policy
set-manifest-pull-policy:
	$(info Updating kustomize pull policy file for default resource)
	sed -i'' -e 's@imagePullPolicy: .*@imagePullPolicy: '"$(PULL_POLICY)"'@' ./config/default/manager_pull_policy.yaml

builder-image-promote-latest:
	./hack/ensure-env-variables.sh USERNAME PASSWORD
	skopeo copy --src-creds=$(USERNAME):$(PASSWORD) --dest-creds=$(USERNAME):$(PASSWORD) \
		docker://$(BUILDER_IMAGE):$(BUILDER_IMAGE_VERSION) \
		docker://$(BUILDER_IMAGE):latest

##@ Binary
##########
# Binary #
##########
$(INFRA_SHORT): ## Build controller binary.
	go build -mod=vendor -o bin/manager main.go

run: ## Run a controller from your host.
	go run ./main.go

##@ Testing
###########
# Testing #
###########
.PHONY: test-unit
test-unit: $(SETUP_ENVTEST) $(GOTESTSUM) ## Run unit and integration tests
	./hack/test-unit.sh

##@ Verify
##########
# Verify #
##########
.PHONY: verify-boilerplate
verify-boilerplate: ## Verify boilerplate text exists in each file
	./hack/verify-boilerplate.sh

.PHONY: verify-shellcheck
verify-shellcheck: ## Verify shell files
	./hack/verify-shellcheck.sh

.PHONY: verify-manifests ## Verify Manifests
verify-manifests:
	./hack/verify-manifests.sh

.PHONY: verify-container-images
verify-container-images: ## Verify container images
	trivy image -q --exit-code 1 --ignore-unfixed --severity MEDIUM,HIGH,CRITICAL $(IMAGE_PREFIX)/$(INFRA_SHORT):latest

##@ Generate
############
# Generate #
############
.PHONY: generate-boilerplate
generate-boilerplate: ## Generates missing boilerplates
	./hack/ensure-boilerplate.sh

# support go modules
generate-modules: ## Generates missing go modules
ifeq ($(BUILD_IN_CONTAINER),true)
	docker run  --rm -t -i \
		-v $(shell go env GOPATH)/pkg:/go/pkg$(MOUNT_FLAGS) \
		-v $(shell pwd):/src/cluster-api-provider-$(INFRA_PROVIDER)$(MOUNT_FLAGS) \
		$(BUILDER_IMAGE):$(BUILDER_IMAGE_VERSION) $@;
else
	./hack/golang-modules-update.sh
endif

generate-modules-ci: generate-modules
	@if ! (git diff --exit-code ); then \
		echo "\nChanges found in generated files"; \
		exit 1; \
	fi

generate-manifests: $(CONTROLLER_GEN) ## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
	$(CONTROLLER_GEN) \
			paths=./api/... \
			paths=./controllers/... \
			crd:crdVersions=v1 \
			rbac:roleName=manager-role \
			output:crd:dir=./config/crd/bases \
			output:webhook:dir=./config/webhook \
			webhook

generate-go-deepcopy: $(CONTROLLER_GEN) ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	$(CONTROLLER_GEN) \
		object:headerFile="./hack/boilerplate/boilerplate.generatego.txt" \
		paths="./api/..."

generate-api-ci: generate-manifests generate-go-deepcopy
	@if ! (git diff --exit-code ); then \
		echo "\nChanges found in generated files"; \
		exit 1; \
	fi

cluster-templates: $(KUSTOMIZE)
	$(KUSTOMIZE) build templates/cluster-templates/hcloud --load-restrictor LoadRestrictionsNone  > templates/cluster-templates/cluster-template.yaml
	$(KUSTOMIZE) build templates/cluster-templates/hcloud --load-restrictor LoadRestrictionsNone  > templates/cluster-templates/cluster-template-hcloud.yaml
	$(KUSTOMIZE) build templates/cluster-templates/hcloud-network --load-restrictor LoadRestrictionsNone  > templates/cluster-templates/cluster-template-hcloud-network.yaml
	$(KUSTOMIZE) build templates/cluster-templates/hetzner-hcloud-control-planes --load-restrictor LoadRestrictionsNone  > templates/cluster-templates/cluster-template-hetzner-hcloud-control-planes.yaml
	$(KUSTOMIZE) build templates/cluster-templates/hetzner-baremetal-control-planes --load-restrictor LoadRestrictionsNone  > templates/cluster-templates/cluster-template-hetzner-baremetal-control-planes.yaml
	$(KUSTOMIZE) build templates/cluster-templates/hetzner-baremetal-control-planes-remediation --load-restrictor LoadRestrictionsNone  > templates/cluster-templates/cluster-template-hetzner-baremetal-control-planes-remediation.yaml

##@ Format
##########
# Format #
##########
.PHONY: format-golang
format-golang: ## Format the Go codebase and run auto-fixers if supported by the linter.
ifeq ($(BUILD_IN_CONTAINER),true)
	docker run  --rm -t -i \
		-v $(shell go env GOPATH)/pkg:/go/pkg$(MOUNT_FLAGS) \
		-v $(shell pwd):/src/cluster-api-provider-$(INFRA_PROVIDER)$(MOUNT_FLAGS) \
		$(BUILDER_IMAGE):$(BUILDER_IMAGE_VERSION) $@;
else
	go version
	golangci-lint version
	golangci-lint run -v --fix
endif

.PHONY: format-yaml
format-yaml: ## Lint YAML files
ifeq ($(BUILD_IN_CONTAINER),true)
	docker run  --rm -t -i \
		-v $(shell go env GOPATH)/pkg:/go/pkg$(MOUNT_FLAGS) \
		-v $(shell pwd):/src/cluster-api-provider-$(INFRA_PROVIDER)$(MOUNT_FLAGS) \
		$(BUILDER_IMAGE):$(BUILDER_IMAGE_VERSION) $@;
else
	yamlfixer --version
	yamlfixer -c .yamllint.yaml .
endif

##@ Lint
########
# Lint #
########
.PHONY: lint-golang
lint-golang: ## Lint Golang codebase
ifeq ($(BUILD_IN_CONTAINER),true)
	docker run  --rm -t -i \
		-v $(shell go env GOPATH)/pkg:/go/pkg$(MOUNT_FLAGS) \
		-v $(shell pwd):/src/cluster-api-provider-$(INFRA_PROVIDER)$(MOUNT_FLAGS) \
		$(BUILDER_IMAGE):$(BUILDER_IMAGE_VERSION) $@;
else
	go version
	golangci-lint version
	golangci-lint run -v
endif

.PHONY: lint-golang-ci
lint-golang-ci:
ifeq ($(BUILD_IN_CONTAINER),true)
	docker run  --rm -t -i \
		-v $(shell go env GOPATH)/pkg:/go/pkg$(MOUNT_FLAGS) \
		-v $(shell pwd):/src/cluster-api-provider-$(INFRA_PROVIDER)$(MOUNT_FLAGS) \
		$(BUILDER_IMAGE):$(BUILDER_IMAGE_VERSION) $@;
else
	go version
	golangci-lint version
	golangci-lint run --out-format=github-actions
endif

.PHONY: lint-yaml
lint-yaml: ## Lint YAML files
ifeq ($(BUILD_IN_CONTAINER),true)
	docker run  --rm -t -i \
		-v $(shell go env GOPATH)/pkg:/go/pkg$(MOUNT_FLAGS) \
		-v $(shell pwd):/src/cluster-api-provider-$(INFRA_PROVIDER)$(MOUNT_FLAGS) \
		$(BUILDER_IMAGE):$(BUILDER_IMAGE_VERSION) $@;
else
	yamllint --version
	yamllint -c .yamllint.yaml --strict .
endif

.PHONY: lint-yaml-ci
lint-yaml-ci:
ifeq ($(BUILD_IN_CONTAINER),true)
	docker run  --rm -t -i \
		-v $(shell go env GOPATH)/pkg:/go/pkg$(MOUNT_FLAGS) \
		-v $(shell pwd):/src/cluster-api-provider-$(INFRA_PROVIDER)$(MOUNT_FLAGS) \
		$(BUILDER_IMAGE):$(BUILDER_IMAGE_VERSION) $@;
else
	yamllint --version
	yamllint -c .yamllint.yaml . --format github
endif

DOCKERFILES=$(shell find . -not \( -path ./hack -prune \) -not \( -path ./vendor -prune \) -type f -regex ".*Dockerfile.*"  | tr '\n' ' ')
.PHONY: lint-dockerfile
lint-dockerfile: ## Lint Dockerfiles
ifeq ($(BUILD_IN_CONTAINER),true)
	docker run  --rm -t -i \
		-v $(shell go env GOPATH)/pkg:/go/pkg$(MOUNT_FLAGS) \
		-v $(shell pwd):/src/cluster-api-provider-$(INFRA_PROVIDER)$(MOUNT_FLAGS) \
		$(BUILDER_IMAGE):$(BUILDER_IMAGE_VERSION) $@;
else
	hadolint --version
	hadolint -t error $(DOCKERFILES)
endif

lint-links: ## Link Checker
ifeq ($(BUILD_IN_CONTAINER),true)
	docker run --rm -t -i \
		-v $(shell pwd):/src/cluster-api-provider-$(INFRA_PROVIDER)$(MOUNT_FLAGS) \
		$(BUILDER_IMAGE):$(BUILDER_IMAGE_VERSION) $@;
else
	@lychee --version
	lychee --verbose --config .lychee.toml ./*.md  ./docs/**/*.md 2>&1 | grep -vP '\[(200|EXCLUDED)\]'
endif

##@ Main Targets
################
# Main Targets #
################
.PHONY: lint
lint: lint-golang lint-yaml lint-dockerfile lint-links ## Lint Codebase

.PHONY: format
format: format-golang format-yaml ## Format Codebase

.PHONY: generate-mocks
generate-mocks: ## Generate Mocks
ifeq ($(BUILD_IN_CONTAINER),true)
	docker run  --rm -t -i \
		-v $(shell go env GOPATH)/pkg:/go/pkg$(MOUNT_FLAGS) \
		-v $(shell pwd):/src/cluster-api-provider-$(INFRA_PROVIDER)$(MOUNT_FLAGS) \
		$(BUILDER_IMAGE):$(BUILDER_IMAGE_VERSION) $@;
else
	cd pkg/services/baremetal/client; go run github.com/vektra/mockery/v2@v2.53.5
	cd pkg/services/hcloud/client; go run github.com/vektra/mockery/v2@v2.53.5 --all
endif

.PHONY: generate
generate: generate-manifests generate-go-deepcopy generate-boilerplate generate-modules generate-mocks ## Generate Files

ALL_VERIFY_CHECKS = boilerplate shellcheck manifests
.PHONY: verify
verify: generate lint $(addprefix verify-,$(ALL_VERIFY_CHECKS)) ## Verify all

.PHONY: modules
modules: generate-modules ## Update go.mod & go.sum

.PHONY: boilerplate
boilerplate: generate-boilerplate ## Ensure that your files have a boilerplate header

.PHONY: builder-image-push
builder-image-push: ## Build $(INFRA_SHORT)-builder to a new version. For more information see README.
	BUILDER_IMAGE=$(BUILDER_IMAGE) ./hack/upgrade-builder-image.sh

.PHONY: test
test: test-unit ## Runs all unit and integration tests.

.PHONY: create-hetzner-installimage-tgz
create-hetzner-installimage-tgz:
	rm -rf data/hetzner-installimage*
	cd data; \
	  installimageurl=$$(curl -sL https://api.github.com/repos/Assertive-Yield/hetzner-installimage/releases/latest | jq -r .assets[].browser_download_url); \
	  echo $$installimageurl; \
	  curl -sSLO $$installimageurl
	@if [ $$(tar -tzf data/hetzner-installimage*tgz| cut -d/ -f1| sort | uniq) != "hetzner-installimage" ]; then \
	   echo "tgz must contain only one directory. And it must be 'hetzner-installimage'."; \
	   exit 1; \
	fi
	@echo
	@echo "============= ↓↓↓↓↓ Now update the version number here ↓↓↓↓↓ ============="
	@git ls-files | xargs grep -E 'hetzner-installimage.*v[0-9]+\.[0-9]+' || true
	@echo "↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑"
