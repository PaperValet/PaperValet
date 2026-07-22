# PaperValet Makefile

VERSION := 0.1.0
BUILD_TIME := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")

BINARY := papervalet
BUILD_DIR := .

LDFLAGS := -ldflags "-X main.version=$(VERSION) -X main.buildTime=$(BUILD_TIME) -X main.gitCommit=$(GIT_COMMIT)"

.PHONY: build clean test lint run dev tidy deps docker docker-multiarch

build:
	go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY) ./cmd/papervalet

build-linux:
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY)-linux-amd64 ./cmd/papervalet

build-multiarch:
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY)-linux-amd64 ./cmd/papervalet
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY)-linux-arm64 ./cmd/papervalet

clean:
	rm -f $(BUILD_DIR)/$(BINARY)*

test:
	go test -v -race -count=1 ./...

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
	docker build -t papervalet:$(VERSION) .
	docker tag papervalet:$(VERSION) papervalet:latest

docker-multiarch:
	docker buildx build --platform linux/amd64,linux/arm64 -t papervalet:$(VERSION) --load .

.PHONY: help
help:
	@echo "Available targets:"
	@echo "  build           - Build binary with version info"
	@echo "  build-linux     - Build for linux/amd64"
	@echo "  build-multiarch - Build for linux/amd64 and linux/arm64"
	@echo "  clean           - Remove binary"
	@echo "  test            - Run tests with race detector"
	@echo "  lint            - Run linter"
	@echo "  run             - Build and run"
	@echo "  dev             - Run without building"
	@echo "  tidy            - Tidy modules"
	@echo "  deps            - Download dependencies"
	@echo "  docker          - Build Docker image (single arch)"
	@echo "  docker-multiarch - Build multi-arch Docker image (needs buildx)"