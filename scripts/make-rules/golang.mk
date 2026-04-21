# ==============================================================================
# Makefile helper functions for golang
#

# Test flags
TEST_FLAGS := -count=1
TEST_TIMEOUT := 10m

# Lint config
GOLANGCI_LINT_CONFIG := $(ROOT_DIR)/.golangci.yaml

# Exclude tests pattern (e.g., "vendor|test")
EXCLUDE_TESTS ?=

# ==============================================================================
# PHONY Targets
# ==============================================================================
.PHONY: go.build go.build.% go.build.multiarch \
        go.install go.install.% \
        go.lint go.lint.% go.fix go.fix.% \
        go.test go.test.% go.test.coverage go.test.coverage.% \
        go.fmt go.fmt.% \
        go.tidy go.tidy.%

# ==============================================================================
# Build targets

## go.build: Build all modules (disabled for library package)
go.build:
	@$(LOG_INFO) "Build is disabled for library package (no cmd/ directories)"
	@$(LOG_INFO) "Use 'make test' or 'make lint' instead"

## go.build.%: Build specific module
go.build.%:
	@$(call resolve-module-path,$*); \
	$(LOG_INFO) "Building $$module_path"; \
	cd "$(ROOT_DIR)/$$module_path" && $(GO_IN_MODULE) build ./...

## go.build.multiarch: Build for multiple platforms
go.build.multiarch:
	@$(LOG_INFO) "Multiarch build is disabled for library package"

# ==============================================================================
# Install targets

## go.install: Install all modules (disabled for library package)
go.install:
	@$(LOG_INFO) "Install is disabled for library package (no cmd/ directories)"
	@$(LOG_INFO) "Use 'make test' or 'make lint' instead"

## go.install.%: Install specific module
go.install.%:
	@$(call resolve-module-path,$*); \
	$(LOG_INFO) "Installing $$module_path"; \
	cd "$(ROOT_DIR)/$$module_path" && $(GO_IN_MODULE) install ./...

# ==============================================================================
# Format Targets
# ==============================================================================
# Applies multiple code formatters in sequence for comprehensive code styling.
#
# Formatter Pipeline:
#   1. gofumpt - Stricter Go formatting (superset of gofmt)
#                - Enforces stricter formatting rules than standard gofmt
#                - Ensures consistency across the codebase
#   2. goimports - Import management and ordering
#                - Automatically removes unused imports
#                - Groups imports into standard, third-party, and local sections
#   3. golines - Line length management
#                - Shortens long lines while preserving code structure
#                - Maximum line length: 100 characters
#
# Usage:
#   make fmt           - Apply all formatters in sequence
#   make fmt.gofumpt   - Apply only gofumpt
#   make fmt.goimports - Apply only goimports
#   make fmt.golines   - Apply only golines
#   make fmt.check     - CI gate to verify formatting (fails if unformatted)
#
# CI Integration:
#   - Use fmt.check in CI/CD pipelines to enforce formatting standards
#   - Run 'make fmt' locally before committing to ensure fmt.check passes
# ==============================================================================

## go.fmt: Format code using all formatters
go.fmt: go.fmt.gofumpt go.fmt.goimports go.fmt.golines

## go.fmt.%: Format using specific tool
go.fmt.gofumpt:
	@$(call require-tool,$(GOFUMPT))
	@$(LOG_INFO) "Formatting with gofumpt"
	@$(call find-go-files,.,vendor) | $(XARGS) $(GOFUMPT) -w

go.fmt.goimports:
	@$(call require-tool,$(GOIMPORTS))
	@$(LOG_INFO) "Formatting with goimports"
	@$(call find-go-files,.,vendor) | $(XARGS) $(GOIMPORTS) -w -local $(PROJECT_NAME)

go.fmt.golines:
	@$(call require-tool,$(GOLINES))
	@$(LOG_INFO) "Formatting with golines"
	@$(call find-go-files,.,vendor) | $(XARGS) $(GOLINES) -w --max-len=100

## go.fmt.check: Check if code is formatted (CI gate)
go.fmt.check:
	@$(call require-tool,$(GOFUMPT))
	@$(LOG_INFO) "Checking code formatting"
	@unformatted=$$($(GOFUMPT) -l . 2>/dev/null | grep -v vendor); \
	if [ -n "$$unformatted" ]; then \
		echo "Code is not formatted. Run 'make fmt'"; \
		echo "$$unformatted"; \
		exit 1; \
	fi

# ==============================================================================
# Lint Targets
# ==============================================================================
# Runs golangci-lint with comprehensive configuration from .golangci.yaml.
#
# Features:
#   - Multi-module support: Lints all discovered go.mod modules
#   - Auto-fix: make fix applies automatic fixes where possible
#   - Configurable exclusions: Use EXCLUDE_TESTS= to skip specific patterns
#   - Custom configuration: Edit .golangci.yaml to enable/disable linters
#
# Usage:
#   make lint                          - Run all linters across all modules
#   make fix                           - Run linters with auto-fix enabled
#   make go.lint.<module>              - Lint specific module
#   make lint EXCLUDE_TESTS="vendor"   - Exclude patterns from linting
#
# CI Integration:
#   - Use 'make lint' in CI/CD pipelines as a quality gate
#   - Use 'make fix' locally to automatically fix linting issues
#   - Configure .golangci.yaml for project-specific rules
#
# Common Workflows:
#   1. Development: make fix            - Fix issues automatically
#   2. Pre-commit:  make lint          - Verify code quality
#   3. CI/CD:       make lint          - Quality gate
# ==============================================================================

## go.lint.ensure-compatible: Ensure golangci-lint is built with compatible Go version
go.lint.ensure-compatible:
	@$(call require-tool,$(GOLANGCI_LINT))
	@$(LOG_INFO) "Checking golangci-lint Go version compatibility"
	@current_go_version=$$($(GO) version | awk '{print $$3}' | sed 's/go//'); \
	current_major=$$(echo $$current_go_version | cut -d. -f1); \
	current_minor=$$(echo $$current_go_version | cut -d. -f2); \
	lint_build_version=$$($(GOLANGCI_LINT) --version 2>/dev/null | grep -oE 'go[0-9]+\.[0-9]+' | head -1 | sed 's/go//'); \
	if [ -n "$$lint_build_version" ]; then \
		lint_major=$$(echo $$lint_build_version | cut -d. -f1); \
		lint_minor=$$(echo $$lint_build_version | cut -d. -f2); \
		if [ "$$lint_major" -lt "$$current_major" ] || \
		   ([ "$$lint_major" -eq "$$current_major" ] && [ "$$lint_minor" -lt "$$current_minor" ]); then \
			$(LOG_WARN) "golangci-lint built with Go $$lint_build_version, rebuilding with Go $$current_go_version"; \
			$(MAKE) install.golangci-lint.rebuild; \
		fi; \
	fi

## go.lint: Run linters for all modules
go.lint: go.lint.ensure-compatible go.lint.check
	@$(LOG_SUCCESS) "All modules linted successfully"

## go.lint.%: Run linters for specific module
go.lint.%:
	@$(call require-tool,$(GOLANGCI_LINT))
	@$(call require-file,$(GOLANGCI_LINT_CONFIG))
	@$(call resolve-module-path,$*); \
	$(LOG_INFO) "Linting $$module_path"; \
	cd "$(ROOT_DIR)/$$module_path" && GOWORK=off $(GOLANGCI_LINT) run --config=$(GOLANGCI_LINT_CONFIG) ./...

## go.lint.check: Check all modules
go.lint.check:
	@$(call require-tool,$(GOLANGCI_LINT))
	@$(call require-file,$(GOLANGCI_LINT_CONFIG))
	$(call validate-module-selection,$(MODULES))
	@$(LOG_INFO) "Running linters"
	@source $(ROOT_DIR)/scripts/lib/logger.sh >/dev/null 2>&1; \
	for module in $(MODULES); do \
		log::info "$$module"; \
		cd "$(ROOT_DIR)/$$module" || exit 1; \
		if [ -n "$(EXCLUDE_TESTS)" ]; then \
			GOWORK=off $(GOLANGCI_LINT) run --config=$(GOLANGCI_LINT_CONFIG) --exclude="$(EXCLUDE_TESTS)" ./... || exit 1; \
		else \
			GOWORK=off $(GOLANGCI_LINT) run --config=$(GOLANGCI_LINT_CONFIG) ./... || exit 1; \
		fi; \
	done

## go.fix: Run linters with auto-fix
go.fix: go.lint.ensure-compatible
	@$(call require-tool,$(GOLANGCI_LINT))
	@$(call require-file,$(GOLANGCI_LINT_CONFIG))
	$(call validate-module-selection,$(MODULES))
	@$(LOG_INFO) "Running linters with auto-fix"
	@source $(ROOT_DIR)/scripts/lib/logger.sh >/dev/null 2>&1; \
	for module in $(MODULES); do \
		log::info "$$module"; \
		cd "$(ROOT_DIR)/$$module" || exit 1; \
		if [ -n "$(EXCLUDE_TESTS)" ]; then \
			GOWORK=off $(GOLANGCI_LINT) run --config=$(GOLANGCI_LINT_CONFIG) --fix --exclude="$(EXCLUDE_TESTS)" ./... || exit 1; \
		else \
			GOWORK=off $(GOLANGCI_LINT) run --config=$(GOLANGCI_LINT_CONFIG) --fix ./... || exit 1; \
		fi; \
	done

## go.fix.%: Fix specific module
go.fix.%:
	@$(call require-tool,$(GOLANGCI_LINT))
	@$(call require-file,$(GOLANGCI_LINT_CONFIG))
	@$(call resolve-module-path,$*); \
	$(LOG_INFO) "Fixing $$module_path"; \
	cd "$(ROOT_DIR)/$$module_path" && GOWORK=off $(GOLANGCI_LINT) run --config=$(GOLANGCI_LINT_CONFIG) --fix ./...

# ==============================================================================
# Test Targets
# ==============================================================================
# Comprehensive testing infrastructure with multiple execution modes.
#
# Execution Modes:
#   - Unit tests:           make test                Run all unit tests
#   - Race detection:       make test.race           Run tests with race detector
#   - Benchmarks:           make test.bench          Run benchmark tests
#   - Coverage analysis:    make coverage            Run tests with coverage quality gate
#
# Module-Specific Testing:
#   - Test single module:   make go.test.<module-name>  Test specific module only
#   - Example:              make go.test.xjwt        Test the xjwt module
#
# Coverage Quality Gate:
#   - Default threshold:    80% (configurable via COVERAGE=)
#   - Usage example:        make coverage COVERAGE=80
#   - Reports location:     $(COVERAGE_DIR)/
#     - <module>.out        Raw coverage data (profiling format)
#     - <module>.html       HTML coverage report (browse in browser)
#
# Test Configuration:
#   - TEST_FLAGS:           Additional flags (default: -count=1)
#   - TEST_TIMEOUT:         Test timeout (default: 10m)
#   - EXCLUDE_TESTS:        Pattern to exclude from tests
#
# Examples:
#   make test                      Run all unit tests
#   make test.race                 Run with race detector
#   make coverage COVERAGE=80      Enforce 80% coverage threshold
#   make go.test.xjwt              Test only the xjwt module
# ==============================================================================

## go.test: Run tests for all modules
go.test: go.test.unit
	@$(LOG_SUCCESS) "All tests passed"

## go.test.unit: Run unit tests
go.test.unit:
	$(call validate-module-selection,$(MODULES))
	@$(LOG_INFO) "Running unit tests"
	@source $(ROOT_DIR)/scripts/lib/logger.sh >/dev/null 2>&1; \
	for module in $(MODULES); do \
		log::info "$$module"; \
		cd "$(ROOT_DIR)/$$module" && $(GO_IN_MODULE) test $(TEST_FLAGS) -timeout=$(TEST_TIMEOUT) ./... || exit 1; \
	done

## go.test.race: Run tests with race detector
go.test.race:
	$(call validate-module-selection,$(MODULES))
	@$(LOG_INFO) "Running tests with race detector"
	@source $(ROOT_DIR)/scripts/lib/logger.sh >/dev/null 2>&1; \
	for module in $(MODULES); do \
		log::info "$$module"; \
		cd "$(ROOT_DIR)/$$module" && $(GO_IN_MODULE) test $(TEST_FLAGS) -race -timeout=$(TEST_TIMEOUT) ./... || exit 1; \
	done

## go.test.coverage: Run tests with coverage quality gate
go.test.coverage: | $(COVERAGE_DIR)
go.test.coverage: go.test.coverage.all go.test.coverage.check

## go.test.coverage.%: Run coverage for specific module
go.test.coverage.%: | $(COVERAGE_DIR)
	@$(call resolve-module-path,$*); \
	module_file=$$(echo "$$module_path" | tr '/' '_'); \
	$(LOG_INFO) "Coverage for $$module_path"; \
	test_pkgs=$$(cd "$(ROOT_DIR)/$$module_path" && $(GO_IN_MODULE) list -f '{{if or .TestGoFiles .XTestGoFiles}}{{.ImportPath}}{{end}}' ./... | sed '/^$$/d'); \
	if [ -z "$$test_pkgs" ]; then \
		$(LOG_WARN) "$$module_path has no test packages, skipping coverage profile"; \
		exit 0; \
	fi; \
	cd "$(ROOT_DIR)/$$module_path" && \
		$(GO_IN_MODULE) test -coverprofile=$(COVERAGE_DIR)/$$module_file.out -covermode=atomic $$test_pkgs && \
		GOWORK=off $(GO) tool cover -html=$(COVERAGE_DIR)/$$module_file.out -o $(COVERAGE_DIR)/$$module_file.html && \
		GOWORK=off $(GO) tool cover -func=$(COVERAGE_DIR)/$$module_file.out | tail -1

# Directory creation rule (order-only prerequisite)
$(COVERAGE_DIR):
	@$(MKDIR) $(COVERAGE_DIR)

## go.test.coverage.all: Generate coverage for all modules
go.test.coverage.all: | $(COVERAGE_DIR)
	$(call validate-module-selection,$(MODULES))
	@$(LOG_INFO) "Generating coverage for all modules"
	@source $(ROOT_DIR)/scripts/lib/logger.sh >/dev/null 2>&1; \
	for module in $(MODULES); do \
		module_file=$$(echo "$$module" | tr '/' '_'); \
		log::info "$$module"; \
		test_pkgs=$$(cd "$(ROOT_DIR)/$$module" && $(GO_IN_MODULE) list -f '{{if or .TestGoFiles .XTestGoFiles}}{{.ImportPath}}{{end}}' ./... | sed '/^$$/d'); \
		if [ -z "$$test_pkgs" ]; then \
			log::warn "$$module has no test packages, skipping coverage profile"; \
			continue; \
		fi; \
		cd "$(ROOT_DIR)/$$module" && \
		$(GO_IN_MODULE) test -coverprofile=$(COVERAGE_DIR)/$$module_file.out -covermode=atomic $$test_pkgs || exit 1; \
		GOWORK=off $(GO) tool cover -html=$(COVERAGE_DIR)/$$module_file.out -o $(COVERAGE_DIR)/$$module_file.html; \
	done
	@$(LOG_SUCCESS) "Coverage reports: $(COVERAGE_DIR)"

## go.test.coverage.check: Check coverage against quality gate
go.test.coverage.check: | $(COVERAGE_DIR)
	$(call validate-module-selection,$(MODULES))
	@$(LOG_INFO) "Checking coverage quality gate (target: $(COVERAGE)%)"
	@set -o pipefail; \
	for module in $(MODULES); do \
		module_file=$$(echo "$$module" | tr '/' '_'); \
		profile_file="$(COVERAGE_DIR)/$$module_file.out"; \
		if [ ! -f "$$profile_file" ]; then \
			$(LOG_WARN) "$$module has no coverage profile, skipping quality gate"; \
			continue; \
		fi; \
		cd "$(ROOT_DIR)/$$module" && GOWORK=off $(GO) tool cover -func=$$profile_file | grep -E "^total:" | \
		awk -v target=$(COVERAGE) -f $(ROOT_DIR)/scripts/coverage.awk || exit 1; \
	done
	@$(LOG_SUCCESS) "Coverage quality gate passed!"

## go.test.%: Run tests for specific module
go.test.%:
	@$(call resolve-module-path,$*); \
	$(LOG_INFO) "Testing $$module_path"; \
	cd "$(ROOT_DIR)/$$module_path" && $(GO_IN_MODULE) test $(TEST_FLAGS) -timeout=$(TEST_TIMEOUT) ./...

## go.test.bench: Run benchmarks
go.test.bench:
	@$(LOG_INFO) "Running benchmarks"
	$(call iterate-modules-log,$(GO_IN_MODULE) test -bench=. -benchmem -timeout=$(TEST_TIMEOUT) ./...)
