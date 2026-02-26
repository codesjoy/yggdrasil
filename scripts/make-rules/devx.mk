# ==============================================================================
# Developer Experience Targets
# ==============================================================================
# Aggregated diagnostics and common check pipelines.
#
# Targets:
#   - modules.print:   Print module discovery and selection context
#   - doctor:          Run environment/tooling/hooks/workspace diagnostics
#   - check.fast:      Run fmt.check + lint + test
#   - check:           Run check.fast + coverage + go.work.drift + go.mod.tidy.check
#   - check.strict:    Run strict checks (examples + race)
# ==============================================================================

.PHONY: modules.print \
        doctor doctor.env doctor.tools doctor.hooks doctor.workspace \
        check.fast check check.strict

## modules.print: Print discovered and selected modules with filter context
modules.print:
	@$(LOG_INFO) "Printing module selection context"
	@echo "Module selection context:"
	@echo "  MODULES_ORIGIN:    $(MODULES_ORIGIN)"
	@echo "  MODULES_EXPLICIT:  $(MODULES_EXPLICIT)"
	@echo "  MODULE_INCLUDE:    $(MODULE_INCLUDE)"
	@echo "  MODULE_EXCLUDE:    $(MODULE_EXCLUDE)"
	@echo "  INCLUDE_EXAMPLES:  $(INCLUDE_EXAMPLES)"
	@echo "  ALL_MODULES_COUNT: $(words $(ALL_MODULES))"
	@echo "  MODULES_COUNT:     $(words $(MODULES))"
	@echo ""
	@echo "ALL_MODULES:"
	@for module in $(ALL_MODULES); do \
		echo "  - $$module"; \
	done
	@echo ""
	@echo "MODULES:"
	@for module in $(MODULES); do \
		echo "  - $$module"; \
	done

## doctor: Run environment, tooling, hooks, and workspace diagnostics
doctor: doctor.env doctor.tools doctor.hooks doctor.workspace
	@$(LOG_SUCCESS) "Doctor checks passed"

## doctor.env: Check required runtime commands and print versions
doctor.env:
	@$(LOG_INFO) "Checking environment"
	@if ! command -v $(GO) >/dev/null 2>&1; then \
		$(LOG_ERROR) "Go not found in PATH"; \
		exit 1; \
	fi
	@if ! command -v make >/dev/null 2>&1; then \
		$(LOG_ERROR) "make not found in PATH"; \
		exit 1; \
	fi
	@$(GO) version
	@make --version | head -n 1

## doctor.tools: Verify configured tools and optional shellcheck
doctor.tools:
	@$(LOG_INFO) "Checking tooling"
	@$(MAKE) --no-print-directory tools.verify
	@if ! command -v $(SHFMT) >/dev/null 2>&1; then \
		$(LOG_ERROR) "shfmt not found. Run 'make tools' to install."; \
		exit 1; \
	fi
	@if command -v shellcheck >/dev/null 2>&1; then \
		$(LOG_INFO) "shellcheck available"; \
	elif [ "$(SHELLCHECK_REQUIRED)" = "1" ]; then \
		$(LOG_ERROR) "shellcheck not found and SHELLCHECK_REQUIRED=1"; \
		exit 1; \
	else \
		$(LOG_WARN) "shellcheck not found, diagnostics continue"; \
	fi

## doctor.hooks: Verify git hooks installation
doctor.hooks:
	@$(LOG_INFO) "Checking git hooks"
	@$(MAKE) --no-print-directory hooks.verify

## doctor.workspace: Verify go.work exists and is in sync
doctor.workspace:
	@$(LOG_INFO) "Checking workspace state"
	@$(MAKE) --no-print-directory go.work.verify
	@$(MAKE) --no-print-directory go.work.drift

## check.fast: Run fast quality gates (fmt.check, lint, test)
check.fast: fmt.check lint test
	@$(LOG_SUCCESS) "Fast checks passed"

## check: Run full quality gates (check.fast + coverage + go.work.drift + go.mod.tidy.check)
check: check.fast coverage go.work.drift go.mod.tidy.check
	@$(LOG_SUCCESS) "Full checks passed"

## check.strict: Run strict quality gates (examples + race)
check.strict:
	@$(LOG_INFO) "Running strict checks (includes example modules)"
	@$(MAKE) --no-print-directory check INCLUDE_EXAMPLES=1
	@$(MAKE) --no-print-directory test.race INCLUDE_EXAMPLES=1
	@$(LOG_SUCCESS) "Strict checks passed"
