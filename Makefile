# Image URL to use all building/pushing image targets
IMG ?= ironcore-csi-driver:latest
# ENVTEST_K8S_VERSION refers to the version of kubebuilder assets to be downloaded by envtest binary.
ENVTEST_K8S_VERSION = 1.31.0

# Setting SHELL to bash allows bash commands to be executed by recipes.
# This is a requirement for 'setup-envtest.sh' in the test target.
# Options are set to exit when a recipe line exits non-zero or a piped command fails.
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

all: check

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

help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

BINARY_NAME=ironcore-csi-driver
DOCKER_IMAGE=ironcore-csi-driver

BUILDARGS ?=

# For Development Build #################################################################
# Docker.io username and tag
DOCKER_USER=ironcore
DOCKER_IMAGE_TAG=latest
# For Development Build #################################################################

clean:
	rm -f $(BINARY_NAME)

build:
	go build -o $(BINARY_NAME) -v  ./cmd/

.PHONY: lint
lint: golangci-lint ## Run golangci-lint on the code.
	$(GOLANGCI_LINT) run ./...

# Cross compilation
build-linux:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o $(BINARY_NAME) -v ./cmd/

docker-build:
	docker build $(BUILDARGS) -t ${IMG} -f Dockerfile . --load

docker-push:
	docker push ${IMG}

deploy:
	cd config/manager && $(KUSTOMIZE) edit set image ironcore-csi-driver=${IMG}
	kubectl apply -k config/default

undeploy:
	kubectl delete -k  config/default

.PHONY: fmt
fmt: goimports ## Run goimports against code.
	$(GOIMPORTS) -w .

vet: ## Run go vet against code.
	go vet ./...

test: generate-mocks fmt vet envtest ## Run tests.
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) --bin-dir $(LOCALBIN) -p path)" go test $$(go list ./... | grep -v /e2e) -coverprofile cover.out

.PHONY: generate-mocks
generate-mocks: mockgen ## Generate code (mocks etc.).
	MOCKGEN=$(MOCKGEN) go generate ./...
	./hack/fix-mountwrapper-mock.sh

.PHONY: add-license
add-license: addlicense ## Add license headers to all go files.
	find . -name '*.go' -exec $(ADDLICENSE) -f hack/license-header.txt {} +

.PHONY: check-license
check-license: addlicense ## Check that every file has a license header present.
	find . -name '*.go' -exec $(ADDLICENSE) -check -c 'IronCore authors' {} +

.PHONY: check
check: add-license fmt lint test # Generate manifests, code, lint, add licenses, test

.PHONY: clean-local-bin
clean-local-bin:
	rm -rf $(LOCALBIN)/*

##@ Tools

## Location to install dependencies to
LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p $(LOCALBIN)

## Tool Binaries
KUSTOMIZE ?= $(LOCALBIN)/kustomize
ENVTEST ?= $(LOCALBIN)/setup-envtest
ADDLICENSE ?= $(LOCALBIN)/addlicense
GOIMPORTS ?= $(LOCALBIN)/goimports
MOCKGEN ?= $(LOCALBIN)/mockgen
GOLANGCI_LINT ?= $(LOCALBIN)/golangci-lint

## Tool Versions
KUSTOMIZE_VERSION ?= v5.5.0
ADDLICENSE_VERSION ?= v1.1.1
MOCKGEN_VERSION ?= v0.5.0
GOIMPORTS_VERSION ?= v0.26.0
GOLANGCI_LINT_VERSION ?= v1.62.0

KUSTOMIZE_INSTALL_SCRIPT ?= "https://raw.githubusercontent.com/kubernetes-sigs/kustomize/master/hack/install_kustomize.sh"
.PHONY: kustomize
kustomize: $(KUSTOMIZE) ## Download kustomize locally if necessary.
$(KUSTOMIZE): $(LOCALBIN)
	@if test -x $(LOCALBIN)/kustomize && ! $(LOCALBIN)/kustomize version | grep -q $(KUSTOMIZE_VERSION); then \
		echo "$(LOCALBIN)/kustomize version is not expected $(KUSTOMIZE_VERSION). Removing it before installing."; \
		rm -rf $(LOCALBIN)/kustomize; \
	fi
	test -s $(LOCALBIN)/kustomize || { curl -Ss $(KUSTOMIZE_INSTALL_SCRIPT) | bash -s -- $(subst v,,$(KUSTOMIZE_VERSION)) $(LOCALBIN); }

.PHONY: envtest
envtest: $(ENVTEST) ## Download envtest-setup locally if necessary.
$(ENVTEST): $(LOCALBIN)
	test -s $(LOCALBIN)/setup-envtest || GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-runtime/tools/setup-envtest@latest

.PHONY: addlicense
addlicense: $(ADDLICENSE) ## Download addlicense locally if necessary.
$(ADDLICENSE): $(LOCALBIN)
	test -s $(LOCALBIN)/addlicense || GOBIN=$(LOCALBIN) go install github.com/google/addlicense@$(ADDLICENSE_VERSION)

.PHONY: goimports
goimports: $(GOIMPORTS) ## Download goimports locally if necessary.
$(GOIMPORTS): $(LOCALBIN)
	test -s $(LOCALBIN)/goimports || GOBIN=$(LOCALBIN) go install golang.org/x/tools/cmd/goimports@$(GOIMPORTS_VERSION)

.PHONY: mockgen
mockgen: $(MOCKGEN) ## Download mockgen locally if necessary.
$(MOCKGEN): $(LOCALBIN)
	test -s $(LOCALBIN)/mockgen || GOBIN=$(LOCALBIN) go install go.uber.org/mock/mockgen@$(MOCKGEN_VERSION)

.PHONY: golangci-lint
golangci-lint: $(GOLANGCI_LINT) ## Download golangci-lint locally if necessary.
$(GOLANGCI_LINT): $(LOCALBIN)
	test -s $(LOCALBIN)/golangci-lint || GOBIN=$(LOCALBIN) go install github.com/golangci/golangci-lint/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)
