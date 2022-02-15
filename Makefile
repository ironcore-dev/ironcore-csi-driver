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
DOCKER_USER=user1
DOCKER_IMAGE_TAG=test1
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


clean:
	$(GOCLEAN)
	rm -f $(BINARY_NAME)

build:
	$(GOBUILD) -o $(BINARY_NAME) -v  ./cmd/ 

test: 
	$(GOTEST) -v ./...
  
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
	docker build -t $(DOCKER_USER)/$(DOCKER_IMAGE):$(DOCKER_IMAGE_TAG) -f Dockerfile .
 
docker-push:
	docker push $(DOCKER_USER)/$(DOCKER_IMAGE):$(DOCKER_IMAGE_TAG)
 
buildlocal: build docker-build clean

all: build docker-build docker-push clean

deploy: all 
# kustomize code to be added
 