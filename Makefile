# PaperValet Makefile

VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "0.1.0")
BUILD_TIME := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")

BINARY := papervalet
BUILD_DIR := .

LDFLAGS := -ldflags "-X main.version=$(VERSION) -X main.buildTime=$(BUILD_TIME) -X main.gitCommit=$(GIT_COMMIT)"

.PHONY: build build-linux build-multiarch clean test lint run dev tidy deps docker docker-multiarch help

build:
	go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY) ./cmd/papervalet

build-linux:
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY)-linux-amd64 ./cmd/papervalet

build-multiarch:
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY)-linux-amd64 ./cmd/papervalet
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY)-linux-arm64 ./cmd/papervalet
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY)-darwin-amd64 ./cmd/papervalet
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY)-darwin-arm64 ./cmd/papervalet
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY)-windows-amd64.exe ./cmd/papervalet

clean:
	rm -f $(BUILD_DIR)/$(BINARY)*

test:
	go test ./...

test-race:
	go test -race ./...

lint:
	golangci-lint run

run: build
	./$(BINARY) -config config.json

dev:
	go run ./cmd/papervalet -config config.json

tidy:
	go mod tidy

deps:
	go mod download

docker:
	docker build -t $(BINARY):$(VERSION) .

docker-multiarch:
	docker buildx build --platform linux/amd64,linux/arm64 -t $(BINARY):$(VERSION) --push .

.PHONY: help
help:
	@echo "Available targets:"
	@echo "  build           - Build binary for current platform"
	@echo "  build-linux     - Build binary for linux/amd64"
	@echo "  build-multiarch - Build binaries for all platforms"
	@echo "  clean           - Remove binaries"
	@echo "  test            - Run tests"
	@echo "  test-race       - Run tests with race detector"
	@echo "  lint            - Run linter"
	@echo "  run             - Build and run"
	@echo "  dev             - Run without building"
	@echo "  tidy            - Tidy modules"
	@echo "  deps            - Download dependencies"
	@echo "  docker          - Build Docker image"
	@echo "  docker-multiarch - Build and push multi-arch Docker image"