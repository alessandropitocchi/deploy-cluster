.PHONY: build test test-e2e lint clean install release

VERSION ?= $(shell git describe --tags --always --dirty)

# Build the binary
build:
	go build -o klastr ./cmd/deploycluster

# Run unit tests
test:
	go test ./... -v

# Run e2e tests (requires Docker and kind)
test-e2e: build
	RUN_E2E=1 go test ./e2e/... -v -timeout 30m

# Run e2e tests in short mode (skip e2e)
test-short:
	go test ./... -v -short

# Run linter
lint:
	golangci-lint run ./...

# Clean build artifacts
clean:
	rm -f klastr
	rm -rf dist/

# Install to GOPATH/bin
install: build
	cp klastr $(GOPATH)/bin/

# Build release binaries for all platforms
release:
	./scripts/release.sh $(VERSION)

# Development helpers
dev-cluster: build
	./klastr init -o /tmp/dev-cluster.yaml
	./klastr run -f /tmp/dev-cluster.yaml

dev-clean: build
	./klastr destroy -f /tmp/dev-cluster.yaml || true
	./klastr destroy --name dev-cluster || true
