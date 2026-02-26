# ==============================================================================
# Shell Script Quality
# ==============================================================================
# Lints repository shell scripts with syntax + formatting + optional shellcheck.
#
# Targets:
#   - scripts.lint: Run bash -n, shfmt -d, and shellcheck -x when available
#
# Configuration:
#   - SHELLCHECK_REQUIRED=0: shellcheck missing only warns (default)
#   - SHELLCHECK_REQUIRED=1: shellcheck missing fails the target
# ==============================================================================

SHELLCHECK ?= shellcheck
SHELLCHECK_REQUIRED ?= 0
SCRIPTS_LINT_ROOT ?= $(ROOT_DIR)/scripts

.PHONY: scripts.lint

## scripts.lint: Lint shell scripts with bash -n, shfmt, and optional shellcheck
scripts.lint:
	@$(call require-tool,$(SHFMT))
	@$(LOG_INFO) "Linting shell scripts"
	@script_files="$$( \
		$(FIND) "$(SCRIPTS_LINT_ROOT)" -type f \( -name "*.sh" -o -path "$(SCRIPTS_LINT_ROOT)/bin/*" \) | sort \
	)"; \
	if [ -z "$$script_files" ]; then \
		$(LOG_WARN) "No shell scripts found under $(SCRIPTS_LINT_ROOT)"; \
		exit 0; \
	fi; \
	for file in $$script_files; do \
		bash -n "$$file" || exit 1; \
	done; \
	$(SHFMT) -d $$script_files || exit 1; \
	if command -v $(SHELLCHECK) >/dev/null 2>&1; then \
		$(SHELLCHECK) -x $$script_files || exit 1; \
	elif [ "$(SHELLCHECK_REQUIRED)" = "1" ]; then \
		$(LOG_ERROR) "shellcheck not found. Install shellcheck or set SHELLCHECK_REQUIRED=0."; \
		exit 1; \
	else \
		$(LOG_WARN) "shellcheck not found, skipping shellcheck validation"; \
	fi; \
	$(LOG_SUCCESS) "Shell scripts linted successfully"
