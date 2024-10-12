# Improved Makefile for Go Basic Blockchain Project
# Organization: Nlaak Studios, LLC
# Version: 2.1

# Project configuration
ORGANIZATION := Nlaak Studios, LLC
MODNAME      := $(shell basename $(shell go list -m))
MODULE       := $(shell go list -m)
VERSION      := $(shell git describe --tags --always --dirty --match=v* 2> /dev/null || echo v0.0.0)
DATE         := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
USEPORT      := 0

# Go related variables
GO           := go
GOPATH       := $(shell go env GOPATH)
GOBIN        := $(GOPATH)/bin
GOLANGCI     := $(GOBIN)/golangci-lint
DELVE        := $(GOBIN)/dlv

# Build variables
BIN          := $(CURDIR)/bin
PKGS         := $(or $(PKG),$(shell $(GO) list ./...))
TESTPKGS     := $(shell $(GO) list -f '{{ if or .TestGoFiles .XTestGoFiles }}{{ .ImportPath }}{{ end }}' $(PKGS))
TIMEOUT      := 15
V            := 0
Q            := $(if $(filter 1,$V),,@)
M            := $(shell printf "\033[34;1m->\033[0m")

# Test coverage variables
COVERAGE_DIR    := $(CURDIR)/test/coverage
COVERAGE_PROFILE := $(COVERAGE_DIR)/profile.out
COVERAGE_XML    := $(COVERAGE_DIR)/coverage.xml
COVERAGE_HTML   := $(COVERAGE_DIR)/index.html

# Development variables
DEV_MAIN     := ./cmd/chaind/main.go
ENV_FILE     := $(CURDIR)/.local.env

.PHONY: all
all: setup fmt lint test build ## Run setup, format, lint, test, and build

.PHONY: build
build: ; $(info $(M) building executable ($(MODNAME))...) @ ## Build production binary
	$Q $(GO) build \
		-tags release \
		-ldflags '-X $(MODULE)/cmd.Version=$(VERSION) -X $(MODULE)/cmd.BuildDate=$(DATE)' \
		-o $(BIN)/$(MODNAME) main.go

.PHONY: run-dev
run-dev: ; $(info $(M) running development version...) @ ## Run development version
	$Q ENV_FILE=$(ENV_FILE) $(GO) run -tags dev $(DEV_MAIN)

.PHONY: debug
debug: ; $(info $(M) debugging development version...) @ ## Debug development version
	$Q $(GO) build -gcflags="all=-N -l" -o $(BIN)/$(MODNAME)-debug $(DEV_MAIN)
	$Q $(DELVE) exec $(BIN)/$(MODNAME)-debug

.PHONY: setup
setup: ; $(info $(M) setting up dependencies...) @ ## Setup go modules
	$Q $(GO) mod init || true
	$Q $(GO) mod tidy
	$Q $(GO) mod vendor

.PHONY: fmt
fmt: ; $(info $(M) running gofmt...) @ ## Run gofmt on all source files
	$Q $(GO) fmt $(PKGS)

.PHONY: lint
lint: | $(GOLANGCI) ; $(info $(M) running golangci-lint...) @ ## Run golangci-lint
	$Q $(GOLANGCI) run

.PHONY: test
test: fmt lint ; $(info $(M) running tests...) @ ## Run tests
	$Q $(GO) test -v -race -cover $(TESTPKGS)

.PHONY: test-coverage
test-coverage: fmt lint ; $(info $(M) running coverage tests...) @ ## Run coverage tests
	$Q mkdir -p $(COVERAGE_DIR)
	$Q $(GO) test \
		-coverpkg=$$($(GO) list -f '{{ join .Deps "\n" }}' $(TESTPKGS) | \
					grep '^$(MODULE)/' | \
					tr '\n' ',' | sed 's/,$$//') \
		-covermode=atomic \
		-coverprofile="$(COVERAGE_PROFILE)" $(TESTPKGS)
	$Q $(GO) tool cover -html=$(COVERAGE_PROFILE) -o $(COVERAGE_HTML)
	$Q $(GO) tool cover -func=$(COVERAGE_PROFILE)

.PHONY: clean
clean: ; $(info $(M) cleaning...) @ ## Clean build artifacts
	@rm -rf $(BIN)
	@rm -rf test
	@rm -rf vendor
	@rm -f go.sum

.PHONY: docker
docker: all ; $(info $(M) building docker image...) @ ## Build docker image
	docker build -t $(MODNAME):$(VERSION) -f Dockerfile .

.PHONY: help
help:
	@echo ""
	@echo "$(ORGANIZATION) Go Basic Blockchain Project"
	@echo "Version: $(VERSION)"
	@echo "Date: $(DATE)"
	@echo "---------------------------------------------------"
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

# Tools
$(GOBIN):
	@mkdir -p $@

$(GOLANGCI): | $(GOBIN) ; $(info $(M) installing golangci-lint...)
	$Q curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(GOBIN) v1.41.1

$(DELVE): | $(GOBIN) ; $(info $(M) installing delve...)
	$Q $(GO) install github.com/go-delve/delve/cmd/dlv@latest

# Cross-compilation targets
.PHONY: build-linux build-windows build-darwin
build-linux: GOOS := linux
build-windows: GOOS := windows
build-darwin: GOOS := darwin
build-linux build-windows build-darwin: ; $(info $(M) building for $(GOOS)...) @ ## Build for specific OS
	$Q GOOS=$(GOOS) $(GO) build \
		-tags release \
		-ldflags '-X $(MODULE)/cmd.Version=$(VERSION) -X $(MODULE)/cmd.BuildDate=$(DATE)' \
		-o $(BIN)/$(MODNAME)-$(GOOS) main.go

# Default target
.DEFAULT_GOAL := help