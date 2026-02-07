.PHONY: build build-all docker-build docker-build-all docker-publish-all clean test lint run

BINARY_NAME := aws-cost-exporter
BUILD_DIR := .build
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -w -s -X main.version=$(VERSION)

# Docker registry
DOCKER_REPO := yohannd/aws-cost-exporter-go
PLATFORMS := linux/amd64,linux/arm64

# ==============================================================================
# Go build targets
# ==============================================================================

build:
	CGO_ENABLED=0 go build -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/exporter

build-all: build-linux-amd64 build-linux-arm64

build-linux-amd64:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/linux-amd64/$(BINARY_NAME) ./cmd/exporter

build-linux-arm64:
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/linux-arm64/$(BINARY_NAME) ./cmd/exporter

# ==============================================================================
# Docker targets
# ==============================================================================

# Build for current architecture only
docker-build: build-linux-amd64
	docker build --build-arg ARCH=amd64 -t $(DOCKER_REPO):$(VERSION) .

# Build multi-arch (local only, not pushed)
docker-build-all: build-all
	docker buildx build \
		--platform $(PLATFORMS) \
		-t $(DOCKER_REPO):$(VERSION) \
		-t $(DOCKER_REPO):latest \
		.

# Build and push multi-arch to registry
docker-publish-all: build-all
	docker buildx build \
		--platform $(PLATFORMS) \
		-t $(DOCKER_REPO):$(VERSION) \
		-t $(DOCKER_REPO):latest \
		--push \
		.

# ==============================================================================
# Development targets
# ==============================================================================

run: build
	./$(BUILD_DIR)/$(BINARY_NAME)

test:
	go test -v ./...

lint:
	golangci-lint run

clean:
	rm -rf $(BUILD_DIR)

# ==============================================================================
# Help
# ==============================================================================

help:
	@echo "Available targets:"
	@echo "  build              - Build binary for current OS/arch"
	@echo "  build-all          - Build binaries for linux/amd64 and linux/arm64"
	@echo "  docker-build       - Build Docker image for amd64"
	@echo "  docker-build-all   - Build multi-arch Docker image (local)"
	@echo "  docker-publish-all - Build and push multi-arch Docker image to registry"
	@echo "  run                - Build and run locally"
	@echo "  test               - Run tests"
	@echo "  lint               - Run linter"
	@echo "  clean              - Remove build artifacts"
