#!/usr/bin/env bash
# Makefile — featcache development workflow.
# Linux is the primary platform; macOS/other platforms build stubs and run tests.

GO       ?= go
GOLANGCI ?= golangci-lint
VERSION  ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT   ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE     ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS   = -s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)
BIN       = bin/featload
COVERAGE_THRESHOLD ?= 70

.PHONY: all build test test-race coverage lint vet fmt check bench clean install-tools release help

all: check

## build: compile the featload binary
build:
	@mkdir -p bin
	$(GO) build -trimpath -ldflags "$(LDFLAGS)" -o $(BIN) ./cmd/featload
	@echo "built $(BIN)"

## test: run all tests with race detector
test:
	$(GO) test ./... -count=1 -race

## test-race: run tests with race detector (alias)
test-race: test

## coverage: measure coverage and enforce threshold
coverage:
	$(GO) test ./... -coverprofile=coverage.out -covermode=atomic -count=1
	$(GO) tool cover -func=coverage.out
	@bash scripts/check_coverage.sh coverage.out $(COVERAGE_THRESHOLD)

## lint: run golangci-lint
lint:
	$(GOLANGCI) run ./...

## vet: run go vet
vet:
	$(GO) vet ./...

## fmt: check gofmt formatting
fmt:
	@test -z "$$(gofmt -l .)" || (echo "gofmt found unformatted files:"; gofmt -l .; exit 1)

## license: verify license headers on all source files
license:
	bash scripts/check_license.sh

## check: run all quality gates (fmt, vet, lint, test, coverage, license)
check: fmt vet lint test coverage license

## bench: run benchmarks
bench:
	$(GO) test ./pkg/featcache/ -bench=. -benchmem -count=1 -timeout=10m

## clean: remove build artifacts
clean:
	rm -rf bin coverage.out

## install-tools: install required development tools
install-tools:
	@command -v $(GOLANGCI) >/dev/null 2>&1 || go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest
	@command -v govulncheck >/dev/null 2>&1 || go install golang.org/x/vuln/cmd/govulncheck@latest
	@echo "tools installed: golangci-lint, govulncheck"

## release: build release artifacts locally via goreleaser (dry-run)
release:
	@command -v goreleaser >/dev/null 2>&1 || (echo "install goreleaser: https://goreleaser.com/install/"; exit 1)
	goreleaser release --clean --skip=publish

## help: show available targets
help:
	@echo "featcache Makefile"
	@echo ""
	@echo "Targets:"
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/## //' | column -t -s ':'
