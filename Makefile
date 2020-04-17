# Project parameters
BINARY_NAME ?= signalilo

BINARY_VERSION = $(shell git describe --tags --always --dirty --match=v* || (echo "command failed $$?"; exit 1))
VERSION ?= $(BINARY_VERSION)

IMAGE_NAME ?= docker.io/vshn/$(BINARY_NAME):$(VERSION)

# Go parameters
GOCMD   ?= go
GOBUILD ?= $(GOCMD) build
GOCLEAN ?= $(GOCMD) clean
GOTEST  ?= $(GOCMD) test
GOGET   ?= $(GOCMD) get

.PHONY: all
all: test build

.PHONY: build
build:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GOBUILD) -v \
		-o $(BINARY_NAME) \
		-ldflags "-w -s -X main.Version=$(VERSION) -X 'main.BuildDate=$(shell date -Iseconds)'"
	@echo built '$(VERSION)'

.PHONY: test
test:
	$(GOTEST) -v -cover ./...

.PHONY: clean
clean:
	$(GOCLEAN)
	rm -f $(BINARY_NAME)
	rm -rf $(web_dir)

.PHONY: docker
docker:
	docker build --build-arg VERSION=$(VERSION) -t $(IMAGE_NAME) .
	@echo built image $(IMAGE_NAME)
