# ==============================================================================
# Makefile helper functions for common variables
#

# Use bash as shell
SHELL := bash

# Project metadata
PROJECT_NAME ?= github.com/codesjoy/yggdrasil/v3
ROOT_DIR     := $(shell pwd)

# Module configuration
# MODULES has highest priority when explicitly provided by caller.
# MODULES_DIR is kept for legacy single-module shorthand targets (e.g. lint.xjwt).
MODULES_DIR ?= cmd

# Module discovery (dynamic) - find all go.mod files and ignore generated/cache dirs
MODULE_DISCOVERY_EXCLUDES ?= vendor _output .tmp .git
ALL_MODULES := $(shell cd "$(ROOT_DIR)" && \
	find . -name "go.mod" -type f \
	$(foreach pattern,$(MODULE_DISCOVERY_EXCLUDES),-not -path "*/$(pattern)/*") \
	-exec dirname {} \; | sed 's|^\./||' | sort)

MODULES        ?= $(ALL_MODULES)
MODULE_INCLUDE ?=
MODULE_EXCLUDE ?=

# Example module filtering (default excludes examples unless explicitly opted in)
EXAMPLE_MODULE_PATTERNS ?= example example/% %/example %/example/% %/examples %/examples/%
INCLUDE_EXAMPLES ?= 0

# Track whether MODULES was explicitly set by caller.
MODULES_ORIGIN := $(origin MODULES)
MODULES_EXPLICIT := 0
ifneq ($(filter command line environment environment override,$(MODULES_ORIGIN)),)
MODULES_EXPLICIT := 1
endif

MODULES_SELECTED := $(strip $(MODULES))

ifneq ($(strip $(MODULE_INCLUDE)),)
MODULES_SELECTED := $(filter $(MODULE_INCLUDE),$(MODULES_SELECTED))
endif

ifneq ($(strip $(MODULE_EXCLUDE)),)
MODULES_SELECTED := $(filter-out $(MODULE_EXCLUDE),$(MODULES_SELECTED))
endif

ifneq ($(strip $(INCLUDE_EXAMPLES)),1)
ifneq ($(MODULES_EXPLICIT),1)
MODULES_SELECTED := $(filter-out $(EXAMPLE_MODULE_PATTERNS),$(MODULES_SELECTED))
endif
endif

MODULES := $(strip $(MODULES_SELECTED))

# Go tools
GO              := go
GO_IN_MODULE    := GOWORK=off $(GO)
GOLANGCI_LINT   := golangci-lint
GOFUMPT         := gofumpt
GOIMPORTS       := goimports
GOLINES         := golines
GIT_CHGLOG      := git-chglog
SHFMT           := shfmt
FIND            := find
XARGS           := xargs

# Build directories
OUTPUT_DIR    := $(ROOT_DIR)/_output
COVERAGE_DIR  := $(OUTPUT_DIR)/coverage

# Coverage threshold (default 80%)
COVERAGE      := 80

# Platform support
PLATFORMS ?= linux_amd64 linux_arm64 darwin_amd64 darwin_arm64
GOOS         := $(shell go env GOOS)
GOARCH       := $(shell go env GOARCH)
PLATFORM     := $(GOOS)_$(GOARCH)

# Tool categorization (defaults, can be overridden)
BLOCKER_TOOLS   ?=
CRITICAL_TOOLS  ?= $(GOLANGCI_LINT) $(GOFUMPT) $(GOIMPORTS) $(GOLINES)
TRIVIAL_TOOLS   ?= $(GIT_CHGLOG) addlicense go-junit-report $(SHFMT)

# Common commands
MKDIR          := mkdir -p
RM             := rm -rf

# ==============================================================================
# Logging Helpers
# ==============================================================================
# Optimized logging using wrapper scripts to avoid repeated shell sourcing.
#
# Available levels: LOG_DEBUG, LOG_INFO, LOG_WARN, LOG_ERROR, LOG_SUCCESS
# Controlled by LOG_LEVEL environment variable (0-3, default: 1)
#
# Usage:
#   @$(LOG_INFO) "Message text"
#   @$(LOG_ERROR) "Error message"
# ==============================================================================

# Logging helpers (optimized - use wrapper scripts to avoid repeated sourcing)
LOG_INFO    = $(ROOT_DIR)/scripts/bin/log-info
LOG_DEBUG   = $(ROOT_DIR)/scripts/bin/log-debug
LOG_WARN    = $(ROOT_DIR)/scripts/bin/log-warn
LOG_ERROR   = $(ROOT_DIR)/scripts/bin/log-error
LOG_SUCCESS = $(ROOT_DIR)/scripts/bin/log-success

# ==============================================================================
# Module Iteration Helpers
# ==============================================================================
# Macros for running commands across multiple modules.
#
# Usage:
#   $(call iterate-modules-log,command)  - Run with per-module logging
#   $(call run-in-modules,command)       - Run without logging
#
# Example:
#   $(call iterate-modules-log,$(GO) test ./...)
# ==============================================================================

# Helper to run command in each module (legacy, without logging)
define run-in-modules
$(call validate-module-selection,$(MODULES))
@for module in $(MODULES); do \
    cd "$(ROOT_DIR)/$$module" && $(1) || exit 1; \
done
endef

# Optimized module iteration with logging (sources logger once per loop)
define iterate-modules-log
$(call validate-module-selection,$(MODULES))
@source $(ROOT_DIR)/scripts/lib/logger.sh >/dev/null 2>&1; \
for module in $(MODULES); do \
    log::info "$$module"; \
    cd "$(ROOT_DIR)/$$module" && $(1) || exit 1; \
done
endef

# Resolve module path for single-module targets.
# Supports explicit module paths (e.g. basic/xjwt) and legacy shorthand with MODULES_DIR (e.g. xjwt).
define resolve-module-path
module_path="$(1)"; \
if [ -d "$(ROOT_DIR)/$$module_path" ] && [ -f "$(ROOT_DIR)/$$module_path/go.mod" ]; then \
	true; \
elif [ -d "$(ROOT_DIR)/$(MODULES_DIR)/$$module_path" ] && [ -f "$(ROOT_DIR)/$(MODULES_DIR)/$$module_path/go.mod" ]; then \
	module_path="$(MODULES_DIR)/$$module_path"; \
else \
	$(LOG_ERROR) "Module '$$module_path' not found. Use a valid module path (e.g. ., cmd/protoc-gen-yggdrasil-rest)."; \
	exit 1; \
fi
endef

# Efficient find with exclusions (avoids piped grep)
# Usage: $(call find-go-files,directory,exclude_patterns...)
# Example: $(call find-go-files,.,vendor _output)
define find-go-files
$(FIND) $(1) -name "*.go" $(foreach pattern,$(2),-not -path "*/$(pattern)/*")
endef

# ==============================================================================
# Validation Helpers
# ==============================================================================

# Validate a list of modules exists and contains go.mod files
# Usage: $(call validate-module-selection,$(MODULES))
define validate-module-selection
@selected_modules="$(strip $(1))"; \
if [ -z "$$selected_modules" ]; then \
	$(LOG_ERROR) "No modules selected. Check MODULES/MODULE_INCLUDE/MODULE_EXCLUDE/INCLUDE_EXAMPLES."; \
	exit 1; \
fi; \
missing_modules=""; \
for module in $$selected_modules; do \
	if [ ! -d "$(ROOT_DIR)/$$module" ] || [ ! -f "$(ROOT_DIR)/$$module/go.mod" ]; then \
		missing_modules="$$missing_modules $$module"; \
	fi; \
done; \
if [ -n "$$missing_modules" ]; then \
	$(LOG_ERROR) "Invalid module(s):$${missing_modules}. Use valid module paths (e.g. ., cmd/protoc-gen-yggdrasil-rest)."; \
	exit 1; \
fi
endef

# Require a tool to be installed
# Usage: $(call require-tool,golangci-lint)
define require-tool
@if ! command -v $(1) >/dev/null 2>&1; then \
    $(LOG_ERROR) "Required tool '$(1)' not found. Run 'make tools' to install."; \
    exit 1; \
fi
endef

# Require a file to exist
# Usage: $(call require-file,$(CONFIG_FILE))
define require-file
@if [ ! -f "$(1)" ]; then \
    $(LOG_ERROR) "Required file '$(1)' not found."; \
    exit 1; \
fi
endef
