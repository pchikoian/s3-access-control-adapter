.PHONY: build test run clean docker-up docker-down lint fmt

BINARY_NAME=gateway
BUILD_DIR=bin

# Detect docker compose command (v1 vs v2)
DOCKER_COMPOSE := $(shell which docker-compose 2>/dev/null)
ifeq ($(DOCKER_COMPOSE),)
	DOCKER_COMPOSE := docker compose
else
	DOCKER_COMPOSE := docker-compose
endif

build:
	go build -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/gateway

test:
	go test -v ./...

test-coverage:
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

run: build
	./$(BUILD_DIR)/$(BINARY_NAME) -config configs/gateway.yaml

clean:
	rm -rf $(BUILD_DIR)
	rm -f coverage.out coverage.html

docker-up:
	$(DOCKER_COMPOSE) up -d

docker-down:
	$(DOCKER_COMPOSE) down

docker-build:
	$(DOCKER_COMPOSE) build

lint:
	golangci-lint run ./...

fmt:
	go fmt ./...

deps:
	go mod download
	go mod tidy
