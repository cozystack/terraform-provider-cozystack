# Makefile for terraform-provider-cozystack
# Run `make help` to list all available targets.

BINARY      := terraform-provider-cozystack
VERSION     ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT      := $(shell git rev-parse HEAD 2>/dev/null || echo "unknown")
LDFLAGS     := -s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT)

.PHONY: build install test test-cover testacc lint lint-fix fmt docs tidy help

##@ Build

build: ## Build the provider binary
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) .

install: ## Build and install the provider into GOBIN (for ~/.terraformrc dev_overrides)
	go install -ldflags "$(LDFLAGS)" .

##@ Testing

test: ## Run unit tests with the race detector
	go test -race ./...

test-cover: ## Run unit tests with a coverage profile (coverage.out)
	go test -race -coverprofile=coverage.out ./...

testacc: ## Run acceptance tests (needs a live Cozystack cluster via KUBECONFIG/KUBE_CTX)
	TF_ACC=1 go test -race -timeout 30m ./internal/provider/...

##@ Linting & formatting

lint: ## Run golangci-lint
	golangci-lint run --timeout=5m

lint-fix: ## Run golangci-lint with auto-fix
	golangci-lint run --timeout=5m --fix

fmt: ## Format Go sources
	gofmt -w .
	gofumpt -w . 2>/dev/null || true
	goimports -w . 2>/dev/null || true

##@ Documentation

docs: ## Regenerate registry docs from schema + examples (needs tfplugindocs)
	tfplugindocs generate --provider-name cozystack

##@ Misc

tidy: ## Tidy go.mod / go.sum
	go mod tidy

help: ## Print this help message
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} \
		/^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-14s\033[0m %s\n", $$1, $$2 } \
		/^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) }' $(MAKEFILE_LIST)
