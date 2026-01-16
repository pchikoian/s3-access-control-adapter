.PHONY: build test run clean docker-up docker-down lint fmt

BINARY_NAME=gateway
BUILD_DIR=bin

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
	docker-compose up -d

docker-down:
	docker-compose down

docker-build:
	docker-compose build

lint:
	golangci-lint run ./...

fmt:
	go fmt ./...

deps:
	go mod download
	go mod tidy
