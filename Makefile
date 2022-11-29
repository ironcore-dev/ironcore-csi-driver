# Image URL to use all building/pushing image targets
IMG ?= onmetal-csi-driver:latest
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
##@ Tools

## Location to install dependencies to
LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p $(LOCALBIN)

## Tool Binaries
KUSTOMIZE ?= $(LOCALBIN)/kustomize

## Tool Versions
KUSTOMIZE_VERSION ?= v3.8.7

KUSTOMIZE_INSTALL_SCRIPT ?= "https://raw.githubusercontent.com/kubernetes-sigs/kustomize/master/hack/install_kustomize.sh"
.PHONY: kustomize
kustomize: $(KUSTOMIZE) ## Download kustomize locally if necessary.
$(KUSTOMIZE): $(LOCALBIN)
	curl -s $(KUSTOMIZE_INSTALL_SCRIPT) | bash -s -- $(subst v,,$(KUSTOMIZE_VERSION)) $(LOCALBIN)

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

test: 
	$(GOTEST) -v ./...

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
	docker build -t ${IMG} -f Dockerfile .
 
docker-push:
	docker push ${IMG}
 
buildlocal: build docker-build clean

all: build docker-build docker-push clean
 
deploy:
	cd config/manager && $(KUSTOMIZE) edit set image onmetal-csi-driver=${IMG}
	kubectl apply -k config/default

undeploy:
	kubectl delete -k  config/default


 
 
