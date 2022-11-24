# Image URL to use all building/pushing image targets
IMG ?= onmetal-csi-driver:latest
# ENVTEST_K8S_VERSION refers to the version of kubebuilder assets to be downloaded by envtest binary.
ENVTEST_K8S_VERSION = 1.24.2

## Location to install dependencies to
LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p $(LOCALBIN)

## Tool Binaries
KUSTOMIZE ?= $(LOCALBIN)/kustomize
#CONTROLLER_GEN ?= $(LOCALBIN)/controller-gen
ENVTEST ?= $(LOCALBIN)/setup-envtest

## Tool Versions
KUSTOMIZE_VERSION ?= v3.8.7
CONTROLLER_TOOLS_VERSION ?= v0.9.2

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOMOD=$(GOCMD) mod
 
BINARY_NAME=onmetal-csi-driver
DOCKER_IMAGE=onmetal-csi-driver

# For Development Build #################################################################
# Docker.io username and tag
DOCKER_USER=onmetal
DOCKER_IMAGE_TAG=latest
# For Development Build #################################################################


# For Production Build ##################################################################
ifeq ($(env),prod)
	# For Production
	# Do not change following values unless change in production version or username
	#For docker.io  
	DOCKER_USER=your_docker_user_name
	DOCKER_IMAGE_TAG=1.1.0
endif
# For Production Build ##################################################################

KUSTOMIZE = $(shell pwd)/bin/kustomize
kustomize: ## Download kustomize locally if necessary.
	$(call go-get-tool,$(KUSTOMIZE),sigs.k8s.io/kustomize/kustomize/v4@v4.4.1)

# go-get-tool will 'go get' any package $2 and install it to $1.
PROJECT_DIR := $(shell dirname $(abspath $(lastword $(MAKEFILE_LIST))))
define go-get-tool
@[ -f $(1) ] || { \
set -e ;\
TMP_DIR=$$(mktemp -d) ;\
cd $$TMP_DIR ;\
go mod init tmp ;\
echo "Downloading $(2)" ;\
GOBIN=$(PROJECT_DIR)/bin go get $(2) ;\
rm -rf $$TMP_DIR ;\
}
endef

clean:
	$(GOCLEAN)
	rm -f $(BINARY_NAME)

build:
	$(GOBUILD) -o $(BINARY_NAME) -v  ./cmd/ 

lint: ## Run golangci-lint against code.
	golangci-lint run ./...

run:
	$(GOBUILD) -o $(BINARY_NAME) -v ./...
	./$(BINARY_NAME)

modverify:
	$(GOMOD) verify

modtidy:
	$(GOMOD) tidy

moddownload:
	$(GOMOD) download

# Cross compilation
build-linux:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GOBUILD) -o $(BINARY_NAME) -v ./cmd/

docker-build: 
	docker build --ssh default=${HOME}/.ssh/id_rsa -t ${IMG} -f Dockerfile .

docker-push:
	docker push ${IMG}

buildlocal: build docker-build clean

all: build docker-build docker-push clean

deploy:
	cd config/manager && $(KUSTOMIZE) edit set image onmetal-csi-driver=${IMG}
	$(KUSTOMIZE) build config/default | kubectl apply -f -

undeploy:
	$(KUSTOMIZE) build config/default | kubectl delete -f -

fmt: ## Run go fmt against code.
	go fmt ./...

vet: ## Run go vet against code.
	go vet ./...

test: fmt vet envtest ## Run tests.
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) -p path)" go test ./... -coverprofile cover.out

envtest: $(ENVTEST) ## Download envtest-setup locally if necessary.
$(ENVTEST): $(LOCALBIN)
	GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-runtime/tools/setup-envtest@latest

