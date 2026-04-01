.PHONY: build test lint docker-build openapi api-client api install-swag

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

## Install swag v2 CLI for OpenAPI generation
install-swag:
	go install github.com/swaggo/swag/v2/cmd/swag@latest

## Generate OpenAPI 3.1 spec from Go annotations
openapi:
	swag init --v3.1 \
		-g cmd/backend/main.go \
		-d .,internal/backend/handler,internal/common/models,internal/operator/api,internal/common/chclient,internal/backend/k8sclient \
		-o docs \
		--outputTypes json,yaml \
		--parseInternal \
		--parseDependency
	mv docs/swagger.json docs/openapi.json
	mv docs/swagger.yaml docs/openapi.yaml

## Generate TypeScript client from OpenAPI spec
api-client:
	cd frontend && npx @hey-api/openapi-ts

## Regenerate OpenAPI spec + TypeScript client
api: openapi api-client
