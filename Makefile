# ==============================================================================
# yggdrasil Makefile

.DEFAULT_GOAL := help

# Include common.mk first (convention)
include scripts/make-rules/common.mk

# Include modular make-rules
include scripts/make-rules/deps.mk
include scripts/make-rules/golang.mk
include scripts/make-rules/copyright.mk
include scripts/make-rules/precommit.mk
include scripts/make-rules/tools.mk
include scripts/make-rules/scripts.mk
include scripts/make-rules/devx.mk
include scripts/make-rules/changelog.mk


# ==============================================================================
# User-facing targets (forward to make-rule targets)

# ==============================================================================
# PHONY Targets
# ==============================================================================
.PHONY: all build install tidy download \
        fmt fmt.check lint fix \
        test test.race test.bench coverage \
        clean copyright tools sync help help.targets \
        hooks.install hooks.verify hooks.run hooks.run-all hooks.clean \
        doctor modules.print scripts.lint check.fast check check.strict \
        changelog changelog.preview changelog.verify \
        changelog.state.print changelog.state.reset

## all: Run format, lint, and test
all: fmt lint test
	@$(LOG_SUCCESS) "All checks passed"

## build: Build (disabled for library package)
build: go.build

## install: Install (disabled for library package)
install: go.install

## tidy: Tidy go.mod for all modules
tidy: go.tidy

## download: Download dependencies
download: go.mod.download

## fmt: Format code using all formatters
fmt: go.fmt

## fmt.check: Check if code is formatted (CI gate)
fmt.check: go.fmt.check

## lint: Run linters
lint: go.lint

## fix: Run linters with auto-fix
fix: go.fix

## test: Run tests for all modules
test: go.test

## test.race: Run tests with race detector
test.race: go.test.race

## test.bench: Run benchmarks
test.bench: go.test.bench

## coverage: Run tests with coverage quality gate
coverage: go.test.coverage

## clean: Clean build artifacts
clean: go.clean

## copyright: Add copyright headers
copyright: copyright.add

## tools: Install all required tools
tools: tools.install

## sync: Sync go workspace
sync: go.work.sync

## help: Show this help message
help:
	@echo ""
	@echo "codesjoy/yggdrasil Makefile"
	@echo ""
	@echo "Usage: make [target] [options]"
	@echo ""
	@echo "Options:"
	@echo "  LOG_LEVEL=0       Show all messages (debug, info, warn, error)"
	@echo "  LOG_LEVEL=1       Show info, warn, error (default)"
	@echo "  LOG_LEVEL=2       Show warn, error only"
	@echo "  LOG_LEVEL=3       Show error only"
	@echo "  COVERAGE=80       Set coverage threshold (default: 80%)"
	@echo "  EXCLUDE_TESTS=    Pattern to exclude from tests (e.g., \"vendor|test\")"
	@echo "  INCLUDE_EXAMPLES=1 Include example/examples modules in lint/fix/test/coverage"
	@echo "  MODULES=...       Explicit module list (e.g., \"cmd/protoc-gen-yggdrasil-rest cmd/protoc-gen-yggdrasil-rpc\"); explicit MODULES are never filtered by INCLUDE_EXAMPLES"
	@echo "  MODULE_INCLUDE=... Filter modules to include (space-separated)"
	@echo "  MODULE_EXCLUDE=... Filter modules to exclude (space-separated)"
	@echo "  MODULES_DIR=cmd   Legacy shorthand base for go.*.<module> targets"
	@echo "  SHELLCHECK_REQUIRED=1 Fail doctor/scripts.lint when shellcheck is missing"
	@echo "  SHFMT_VERSION=...    Override shfmt tool install version"
	@echo "  GIT_CHGLOG_VERSION=... Override git-chglog install version"
	@echo "  CHANGELOG_QUERY=...  Explicit changelog query (tag/SHA range, e.g., v0.1.0..v0.2.0)"
	@echo "  CHANGELOG_FROM=...   Changelog range start ref (tag/SHA, pairs with CHANGELOG_TO)"
	@echo "  CHANGELOG_TO=...     Changelog range end ref (tag/SHA, pairs with CHANGELOG_FROM)"
	@echo "  CHANGELOG_PATHS=...  Space-separated path filters for changelog commits"
	@echo "  CHANGELOG_NEXT_TAG=... Fallback version label when no git tags (default: unreleased)"
	@echo "  CHANGELOG_PROFILE=... simple|balanced|high-frequency (default: balanced)"
	@echo "  CHANGELOG_CADENCE=... monthly|weekly|none (explicitly overrides profile)"
	@echo "  CHANGELOG_USE_BASELINE=1 Use BASE_SHA incremental range in managed mode"
	@echo "  CHANGELOG_ARCHIVE_ENABLE=1 Enable archive bucket rollover in managed mode"
	@echo "  CHANGELOG_STATE_FILE=... Changelog state file path (default: .chglog/state.env)"
	@echo "  CHANGELOG_ARCHIVE_DIR=... Archive section directory (default: .chglog/archive)"
	@echo "  CHANGELOG_NOW=... Test-only time override (e.g., 2026-03-01)"
	@echo "  CHANGELOG_STRICT_STATE=1 Fail when state file is malformed"
	@echo ""
	@echo "Examples:"
	@echo "  make help                     Show this help message"
	@echo "  make tidy                     Tidy dependencies"
	@echo "  make fmt                      Format code"
	@echo "  make lint                     Run linters (unified strict profile)"
	@echo "  make test                     Run tests"
	@echo "  make coverage                 Run tests with coverage"
	@echo "  make check.fast               Run fmt.check + lint + test"
	@echo "  make check                    Run full checks (check.fast + coverage + go.work.drift + go.mod.tidy.check)"
	@echo "  make check.strict             Run strict checks (check + race + examples)"
	@echo "  make test INCLUDE_EXAMPLES=1  Run tests including example modules"
	@echo "  make coverage INCLUDE_EXAMPLES=1 Run coverage including example modules"
	@echo "  make tools                    Install all required tools and pre-commit hooks"
	@echo "  make scripts.lint             Lint shell scripts (bash -n + shfmt + optional shellcheck)"
	@echo "  make doctor                   Run environment/tooling/hooks/workspace diagnostics"
	@echo "  make modules.print            Print module discovery/selection context"
	@echo "  make changelog                Generate CHANGELOG.md"
	@echo "  make changelog.preview        Preview changelog in stdout"
	@echo "  make changelog.verify         Verify CHANGELOG.md is up to date"
	@echo "  make changelog.state.print    Print changelog profile/state/query context"
	@echo "  make changelog.state.reset    Reset changelog baseline state to HEAD"
	@echo "  make hooks.install            Install pre-commit hooks manually"
	@echo "  make hooks.run                Run hooks on staged files"
	@echo "  make hooks.run-all            Run hooks on all files"
	@echo "  make copyright                Add copyright headers"
	@echo "  make sync                     Sync go workspace"
	@echo "  make go.work.drift            Check whether go.work is in sync with discovered modules"
	@echo "  make go.mod.tidy.check        Check whether go.mod/go.sum need tidy updates"
	@echo "  make MODULES=\"cmd/protoc-gen-yggdrasil-rest\" lint"
	@echo "  make MODULE_EXCLUDE=\"example example/protogen\" test"
	@echo "  make changelog CHANGELOG_FROM=v0.1.0 CHANGELOG_TO=v0.2.0"
	@echo "  make changelog.preview CHANGELOG_PATHS=\"basic/xkafka\""
	@echo "  make changelog CHANGELOG_PROFILE=high-frequency"
	@echo "  make changelog.preview CHANGELOG_NOW=2026-03-01"
	@echo ""
	@echo "Module-specific targets:"
	@echo "  make go.test.cmd/protoc-gen-yggdrasil-rest  Test a specific module"
	@echo "  make MODULES=\"cmd/protoc-gen-yggdrasil-rest\" test"
	@echo "  make go.lint.cmd/protoc-gen-yggdrasil-rpc   Lint a specific module"
	@echo ""
	@echo "Available targets:"
	@$(MAKE) help.targets
	@echo ""

## help.targets: Show all targets with descriptions
help.targets:
	@grep -E '^## ' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  %-30s %s\n", $$2, $$3}'
