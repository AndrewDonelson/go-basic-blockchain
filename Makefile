# Improved Makefile for Go Basic Blockchain Project
# Organization: Nlaak Studios, LLC
# Version: 2.2

# OS Detection and path normalization
ifeq ($(OS),Windows_NT)
    # Windows-specific settings
    PATHSEP := \\
    RM := rmdir /s /q
    MKDIR := mkdir
    # Function to normalize paths for Windows
    normalize_path = $(subst /,$(PATHSEP),$1)
    EXE := .exe
else
    # Unix-specific settings
    PATHSEP := /
    RM := rm -rf
    MKDIR := mkdir -p
    # Keep paths as-is on Unix
    normalize_path = $1
    EXE :=
endif

# Project configuration
ORGANIZATION := Nlaak Studios, LLC
MODNAME      := $(shell basename $(shell go list -m))
MODULE       := $(shell go list -m)
VERSION      := $(shell git describe --tags --always --dirty --match=v* 2> /dev/null || echo v0.0.0)
DATE         := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
USEPORT      := 0

# Go related variables
GO           := go
GOPATH       := $(shell $(GO) env GOPATH)
GOBIN        := $(shell $(GO) env GOBIN)
# If GOBIN is empty, use GOPATH/bin
ifeq ($(GOBIN),)
    GOBIN    := $(GOPATH)/bin
endif
GOLANGCI     := $(call normalize_path,$(GOBIN)/golangci-lint$(EXE))
DELVE        := $(call normalize_path,$(GOBIN)/dlv$(EXE))
GOLANGCI_VER := v1.55.2

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

# Architecture support
GOARCH_LIST  := amd64 arm64
GOOS_LIST    := linux windows darwin

.PHONY: all
all: setup fmt lint test build ## Run setup, format, lint, test, and build

.PHONY: build
build: ; $(info $(M) building executable ($(MODNAME))...) @ ## Build production binary
	$Q $(GO) build \
		-mod=mod \
		-tags release \
		-ldflags '-X $(MODULE)/cmd.Version=$(VERSION) -X $(MODULE)/cmd.BuildDate=$(DATE)' \
		-o $(BIN)/$(MODNAME)$(EXE) $(DEV_MAIN)

.PHONY: run-dev
run-dev: ; $(info $(M) running development version...) @ ## Run development version
	$Q ENV_FILE=$(ENV_FILE) $(GO) run -tags dev $(DEV_MAIN)

.PHONY: demo
demo: ; $(info $(M) running progress indicator demo...) @ ## Run progress indicator demo
	$Q $(GO) run ./cmd/demo/main.go

.PHONY: debug
debug: ; $(info $(M) debugging development version...) @ ## Debug development version
	$Q $(GO) build -gcflags="all=-N -l" -o $(BIN)/$(MODNAME)-debug$(EXE) $(DEV_MAIN)
	$Q $(DELVE) exec $(BIN)/$(MODNAME)-debug$(EXE)

.PHONY: setup
setup: | $(GOBIN) ; $(info $(M) setting up dependencies...) @ ## Setup go modules
	$Q if [ ! -f go.mod ]; then $(GO) mod init; fi
	$Q $(GO) mod tidy
	$Q $(GO) mod vendor

.PHONY: fmt
fmt: ; $(info $(M) running gofmt...) @ ## Run gofmt on all source files
	$Q $(GO) fmt $(PKGS)

.PHONY: lint
lint: ; $(info $(M) running golangci-lint...) @ ## Run golangci-lint
ifeq ($(OS),Windows_NT)
	$Q $(GO) install github.com/golangci/golangci-lint/cmd/golangci-lint@$(GOLANGCI_VER)
	$Q $(GOLANGCI) run --timeout 5m
else
	$Q curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b "$(GOBIN)" $(GOLANGCI_VER)
	$Q $(GOLANGCI) run --timeout 5m
endif

.PHONY: test
test: fmt ; $(info $(M) running tests...) @ ## Run tests
	$Q $(GO) test -v -race -cover -timeout 60s $(TESTPKGS)

.PHONY: test-short
test-short: ; $(info $(M) running short tests...) @ ## Run tests in short mode
	$Q $(GO) test -v -short -timeout 60s $(TESTPKGS)

.PHONY: test-unit
test-unit: ; $(info $(M) running unit tests...) @ ## Run only unit tests
	$Q $(GO) test -v -run 'Unit' -timeout 60s $(TESTPKGS)

.PHONY: test-integration
test-integration: ; $(info $(M) running integration tests...) @ ## Run only integration tests
	$Q $(GO) test -v -run 'Integration' -timeout 60s $(TESTPKGS)

.PHONY: bench
bench: ; $(info $(M) running benchmarks...) @ ## Run benchmarks
	$Q $(GO) test -run=^$$ -bench=. -benchmem -timeout 60s $(TESTPKGS)

.PHONY: test-coverage
test-coverage: ; $(info $(M) running coverage tests...) @ ## Run coverage tests
	$Q $(MKDIR) $(COVERAGE_DIR)
	$Q $(GO) test \
		-coverpkg=$$($(GO) list -f '{{ join .Deps "\n" }}' $(TESTPKGS) | \
					grep '^$(MODULE)/' | \
					tr '\n' ',' | sed 's/,$$//') \
		-covermode=atomic \
		-coverprofile="$(COVERAGE_PROFILE)" -timeout 60s $(TESTPKGS)
	$Q $(GO) tool cover -html=$(COVERAGE_PROFILE) -o $(COVERAGE_HTML)
	$Q $(GO) tool cover -func=$(COVERAGE_PROFILE)

.PHONY: clean
clean: ; $(info $(M) cleaning...) @ ## Clean build artifacts
ifeq ($(OS),Windows_NT)
	$Q if exist $(BIN) $(RM) $(BIN)
	$Q if exist test $(RM) test
	$Q if exist vendor $(RM) vendor
	$Q if exist go.sum del go.sum
else
	$Q $(RM) $(BIN)
	$Q $(RM) test
	$Q $(RM) vendor
	$Q $(RM) go.sum
endif

.PHONY: docker
docker: all ; $(info $(M) building docker image...) @ ## Build docker image
	docker build -t $(MODNAME):$(VERSION) -f Dockerfile .

.PHONY: release
release: clean all build-all ; $(info $(M) creating release...) @ ## Create a new release
	$Q $(MKDIR) release
	$Q cp $(BIN)/* release/
	$Q tar -czvf release/$(MODNAME)-$(VERSION).tar.gz -C release $(MODNAME)*
	$Q echo "Created release $(VERSION)"

.PHONY: cross
cross: build-all ## Build for all platforms

.PHONY: verify
verify: ; $(info $(M) verifying dependencies...) @ ## Verify dependencies
	$Q $(GO) mod verify
	$Q $(GO) version

.PHONY: security-scan
security-scan: ; $(info $(M) running security scan...) @ ## Run security scan
	$Q $(GO) install github.com/securego/gosec/v2/cmd/gosec@latest
	$Q $(GOBIN)/gosec$(EXE) -no-fail ./...

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
ifeq ($(OS),Windows_NT)
	$Q if not exist "$(GOBIN)" mkdir "$(GOBIN)"
else
	$Q mkdir -p $@
endif

# Documentation generation targets
.PHONY: docs docs-serve

# Generate documentation
docs: ; $(info $(M) generating documentation...) @ ## Generate project documentation
	$Q $(MKDIR) $(CURDIR)/docs
	integration$Q godoc -url . -html > $(CURDIR)/docs/index.html
	$Q find ./sdk -name "*.go" -not -path "*/test*" | xargs godoc -url | sed 's|/pkg/|./|g' > $(CURDIR)/docs/sdk.html
	$Q echo "Generating documentation overview..."
	$Q $(GO) run scripts/generate_docs.go

# Serve documentation locally
docs-serve: docs ; $(info $(M) serving documentation...) @ ## Serve documentation locally
	$Q godoc -http=:6060

# Cross-compilation targets
.PHONY: build-all
build-all: ## Build for all platforms and architectures
	$(foreach os,$(GOOS_LIST),\
		$(foreach arch,$(GOARCH_LIST),\
			$(info $(M) Building for $(os)/$(arch)...)\
			$(shell GOOS=$(os) GOARCH=$(arch) $(GO) build \
				-mod=mod \
				-tags release \
				-ldflags '-X $(MODULE)/cmd.Version=$(VERSION) -X $(MODULE)/cmd.BuildDate=$(DATE)' \
				-o $(BIN)/$(MODNAME)-$(os)-$(arch)$(if $(filter windows,$(os)),.exe,) $(DEV_MAIN))))

# Individual OS+architecture targets
.PHONY: build-linux-amd64 build-linux-arm64 build-windows-amd64 build-windows-arm64 build-darwin-amd64 build-darwin-arm64
build-linux-amd64: GOOS := linux
build-linux-amd64: GOARCH := amd64
build-linux-arm64: GOOS := linux
build-linux-arm64: GOARCH := arm64
build-windows-amd64: GOOS := windows
build-windows-amd64: GOARCH := amd64
build-windows-arm64: GOOS := windows
build-windows-arm64: GOARCH := arm64
build-darwin-amd64: GOOS := darwin
build-darwin-amd64: GOARCH := amd64
build-darwin-arm64: GOOS := darwin
build-darwin-arm64: GOARCH := arm64

build-linux-amd64 build-linux-arm64 build-windows-amd64 build-windows-arm64 build-darwin-amd64 build-darwin-arm64: ; $(info $(M) building for $(GOOS)/$(GOARCH)...) @ ## Build for specific OS/architecture
	$Q GOOS=$(GOOS) GOARCH=$(GOARCH) $(GO) build \
		-mod=mod \
		-tags release \
		-ldflags '-X $(MODULE)/cmd.Version=$(VERSION) -X $(MODULE)/cmd.BuildDate=$(DATE)' \
		-o $(BIN)/$(MODNAME)-$(GOOS)-$(GOARCH)$(if $(filter windows,$(GOOS)),.exe,) $(DEV_MAIN)

# Default target
.DEFAULT_GOAL := help