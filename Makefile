# Makefile for logs receiver

# Variables
GOPATH ?= $(shell go env GOPATH)
GOBIN ?= $(GOPATH)/bin
GO = go

# Default target
.DEFAULT_GOAL := all

# Build the receiver
.PHONY: build
build:
	$(GO) build ./...

# Run tests
.PHONY: test
test:
	$(GO) test ./...

# Run tests with coverage
.PHONY: test-coverage
test-coverage:
	$(GO) test -race -coverprofile=coverage.out ./...
	$(GO) tool cover -html=coverage.out -o coverage.html

# Format code
.PHONY: fmt
fmt:
	$(GO) fmt ./...

# Lint code
.PHONY: lint
lint:
	golangci-lint run

# Vet code
.PHONY: vet
vet:
	$(GO) vet ./...

# Clean build artifacts
.PHONY: clean
clean:
	$(GO) clean ./...
	rm -f coverage.out coverage.html

# Run all checks
.PHONY: all
all: fmt vet build test

# Install dependencies
.PHONY: deps
deps:
	$(GO) mod download
	$(GO) mod tidy

# Generate code (if needed)
.PHONY: generate
generate:
	$(GO) generate ./...

# Check for security issues
.PHONY: security
security:
	gosec ./...

.PHONY: help
help:
	@echo "Available targets:"
	@echo "  build         - Build the receiver"
	@echo "  test          - Run tests"
	@echo "  test-coverage - Run tests with coverage"
	@echo "  fmt           - Format code"
	@echo "  lint          - Lint code"
	@echo "  vet           - Vet code"
	@echo "  clean         - Clean build artifacts"
	@echo "  all           - Run fmt, vet, build, and test"
	@echo "  deps          - Install dependencies"
	@echo "  generate      - Generate code"
	@echo "  security      - Check for security issues"
	@echo "  help          - Show this help"
