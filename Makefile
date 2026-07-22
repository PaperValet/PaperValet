# PaperValet Makefile

VERSION := 0.1.0
BUILD_TIME := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")

BINARY := papervalet
BUILD_DIR := .

LDFLAGS := -ldflags "-X main.version=$(VERSION) -X main.buildTime=$(BUILD_TIME) -X main.gitCommit=$(GIT_COMMIT)"

.PHONY: build clean test lint run

build:
	go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY) ./cmd/papervalet

clean:
	rm -f $(BUILD_DIR)/$(BINARY)

test:
	go test ./...

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

.PHONY: help
help:
	@echo "Available targets:"
	@echo "  build  - Build binary with version info"
	@echo "  clean  - Remove binary"
	@echo "  test   - Run tests"
	@echo "  lint   - Run linter"
	@echo "  run    - Build and run"
	@echo "  dev    - Run without building"
	@echo "  tidy   - Tidy modules"
	@echo "  deps   - Download dependencies"