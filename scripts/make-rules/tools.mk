# ==============================================================================
# Makefile helper functions for tools
#

# Tool categorization inherited from common.mk
# Default categorization (can be overridden in common.mk or via environment):
#   BLOCKER_TOOLS   - Tools that must be present (currently none)
#   CRITICAL_TOOLS  - Essential tools for CI/CD (linters, formatters)
#   TRIVIAL_TOOLS   - Optional tools (changelog generators, reporters)
#
# Override examples:
#   make BLOCKER_TOOLS=git tools.install
#   make CRITICAL_TOOLS="golangci-lint gofumpt" tools.install

TOOLS ?= $(BLOCKER_TOOLS) $(CRITICAL_TOOLS) $(TRIVIAL_TOOLS)

# Go install path
GOBIN := $(shell go env GOPATH)/bin
PATH  := $(GOBIN):$(PATH)

# Critical tool versions (overridable for reproducible installs)
GOLANGCI_LINT_VERSION ?= v2.7.2
GOFUMPT_VERSION       ?= v0.9.2
GOIMPORTS_VERSION     ?= v0.42.0
GOLINES_VERSION       ?= v0.13.0
SHFMT_VERSION         ?= v3.12.0
GIT_CHGLOG_VERSION    ?= latest

# ==============================================================================
# PHONY Targets
# ==============================================================================
.PHONY: tools.install tools.verify tools.list tools.install.% \
        install.golangci-lint install.golangci-lint.rebuild install.gofumpt install.goimports install.golines \
        install.git-chglog install.addlicense install.go-junit-report install.shfmt

## tools.install: Install all tools and pre-commit hooks
tools.install: $(addprefix tools.install., $(TOOLS)) hooks.install
	@$(LOG_SUCCESS) "All tools and pre-commit hooks installed"

## tools.verify: Verify all tools are installed
tools.verify: $(addprefix tools.verify., $(TOOLS))
	@$(LOG_SUCCESS) "All tools verified"

## tools.verify.%: Verify specific tool (auto-install if missing)
tools.verify.%:
	@$(LOG_INFO) "Verifying tool: $*"
	@if ! command -v $* >/dev/null 2>&1; then \
		$(LOG_WARN) "$* not found, installing..."; \
		$(MAKE) tools.install.$*; \
	fi

## tools.install.%: Install specific tool
tools.install.%:
	@$(LOG_INFO) "Installing $*"
	@$(MAKE) install.$*

# ==============================================================================
# Tool Installation Targets
# ==============================================================================
# Installs development tools using Go's toolchain.
#
# Tool Categories:
#   - CRITICAL_TOOLS:  Linters and formatters (golangci-lint, gofumpt, etc.)
#   - TRIVIAL_TOOLS:   Optional utilities (git-chglog, addlicense, etc.)
#
# Installation:
#   make tools           - Install all categorized tools
#   make tools.install   - Alias for tools
#   make tools.verify    - Check if tools are installed (auto-installs missing)
#   make tools.list      - Show all tools and installation status
#
# Individual tool installation:
#   make install.golangci-lint
#   make install.gofumpt
#   etc.
# ==============================================================================

# Tool installation targets

## install.golangci-lint: Install golangci-lint
install.golangci-lint:
	@$(LOG_INFO) "Installing golangci-lint"
	@$(GO) install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)
	@echo ""
	@echo "golangci-lint installed successfully!"
	@echo ""
	@echo "To enable bash completion, add the following to your ~/.bashrc:"
	@echo "  source \$$(go env GOPATH)/.golangci-lint.bash"
	@echo "Or run this one-time command:"
	@echo "  echo \"source \$$(go env GOPATH)/.golangci-lint.bash\" >> ~/.bashrc && source ~/.bashrc"

## install.golangci-lint.rebuild: Force rebuild golangci-lint with current Go version
install.golangci-lint.rebuild:
	@$(LOG_INFO) "Rebuilding golangci-lint with current Go version"
	@$(GO) install -a github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)

## install.gofumpt: Install gofumpt
install.gofumpt:
	@$(LOG_INFO) "Installing gofumpt"
	@$(GO) install mvdan.cc/gofumpt@$(GOFUMPT_VERSION)

## install.goimports: Install goimports
install.goimports:
	@$(LOG_INFO) "Installing goimports"
	@$(GO) install golang.org/x/tools/cmd/goimports@$(GOIMPORTS_VERSION)

## install.golines: Install golines
install.golines:
	@$(LOG_INFO) "Installing golines"
	@$(GO) install github.com/segmentio/golines@$(GOLINES_VERSION)

## install.shfmt: Install shfmt
install.shfmt:
	@$(LOG_INFO) "Installing shfmt"
	@$(GO) install mvdan.cc/sh/v3/cmd/shfmt@$(SHFMT_VERSION)

## install.git-chglog: Install git-chglog
install.git-chglog:
	@$(LOG_INFO) "Installing git-chglog"
	@$(GO) install github.com/git-chglog/git-chglog/cmd/git-chglog@$(GIT_CHGLOG_VERSION)

## install.addlicense: Install addlicense
install.addlicense:
	@$(LOG_INFO) "Installing addlicense"
	@$(GO) install github.com/google/addlicense@latest

## install.go-junit-report: Install go-junit-report
install.go-junit-report:
	@$(LOG_INFO) "Installing go-junit-report"
	@$(GO) install github.com/jstemmer/go-junit-report/v2@latest

## tools.list: List all tools
tools.list:
	@echo "Blocker tools: $(BLOCKER_TOOLS)"
	@echo "Critical tools: $(CRITICAL_TOOLS)"
	@echo "Trivial tools: $(TRIVIAL_TOOLS)"
	@echo "Critical versions: golangci-lint=$(GOLANGCI_LINT_VERSION) gofumpt=$(GOFUMPT_VERSION) goimports=$(GOIMPORTS_VERSION) golines=$(GOLINES_VERSION)"
	@echo "Trivial versions: git-chglog=$(GIT_CHGLOG_VERSION) shfmt=$(SHFMT_VERSION)"
	@echo ""
	@echo "Installation status:"
	@for tool in $(TOOLS); do \
		if command -v $$tool >/dev/null 2>&1; then \
			echo "  ✓ $$tool"; \
		else \
			echo "  ✗ $$tool (not installed)"; \
		fi \
	done
