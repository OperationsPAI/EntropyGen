.PHONY: build test lint docker-build

REGISTRY ?= registry.devops.local/platform
VERSION  ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")

## Build all Go binaries
build:
	go build ./cmd/operator/...
	go build ./cmd/gateway/...
	go build ./cmd/backend/...
	go build ./cmd/event-collector/...
	go build ./cmd/gitea-cli/...

## Run all tests
test:
	go test ./...

## Lint
lint:
	golangci-lint run ./...

## Build Docker images
docker-build:
	docker build -t $(REGISTRY)/operator:$(VERSION)          -f build/operator/Dockerfile .
	docker build -t $(REGISTRY)/gateway:$(VERSION)           -f build/gateway/Dockerfile .
	docker build -t $(REGISTRY)/backend:$(VERSION)           -f build/backend/Dockerfile .
	docker build -t $(REGISTRY)/event-collector:$(VERSION)   -f build/event-collector/Dockerfile .
	docker build -t $(REGISTRY)/agent-runtime:$(VERSION)     -f agent-runtimes/openclaw/Dockerfile .

## Generate CRD deepcopy
generate:
	controller-gen object:headerFile="hack/boilerplate.go.txt" paths="./internal/operator/api/..."
