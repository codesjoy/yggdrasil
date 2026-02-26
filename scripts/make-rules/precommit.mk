# ==============================================================================
# Pre-commit Hook Management
# ==============================================================================
# Manages pre-commit installation and hook lifecycle.
#
# Targets:
#   - hooks.install: Install pre-commit and register git hooks
#   - hooks.verify:  Verify hooks are installed in .git/hooks
#   - hooks.run:     Run hooks on staged files
#   - hooks.run-all: Run hooks on all files
#   - hooks.clean:   Uninstall pre-commit git hooks
# ==============================================================================

PRE_COMMIT      := pre-commit
PRE_COMMIT_FILE := $(ROOT_DIR)/.pre-commit-config.yaml
PYTHON3         := python3

.PHONY: hooks.install hooks.verify hooks.run hooks.run-all hooks.clean

define resolve-pre-commit-bin
PRE_COMMIT_BIN="$$(command -v $(PRE_COMMIT) || true)"; \
if [ -z "$$PRE_COMMIT_BIN" ] && command -v $(PYTHON3) >/dev/null 2>&1; then \
	USER_PRE_COMMIT="$$( $(PYTHON3) -m site --user-base 2>/dev/null )/bin/$(PRE_COMMIT)"; \
	if [ -x "$$USER_PRE_COMMIT" ]; then \
		PRE_COMMIT_BIN="$$USER_PRE_COMMIT"; \
	fi; \
fi; \
if [ -z "$$PRE_COMMIT_BIN" ]; then \
	$(LOG_ERROR) "pre-commit not found. Run 'make hooks.install' first."; \
	exit 1; \
fi
endef

## hooks.install: Install pre-commit and register git hooks
hooks.install:
	@$(LOG_INFO) "Installing pre-commit hooks"
	@if [ ! -f "$(PRE_COMMIT_FILE)" ]; then \
		$(LOG_ERROR) "Missing $(PRE_COMMIT_FILE)"; \
		exit 1; \
	fi
	@PRE_COMMIT_BIN="$$(command -v $(PRE_COMMIT) || true)"; \
	if [ -z "$$PRE_COMMIT_BIN" ]; then \
		if ! command -v $(PYTHON3) >/dev/null 2>&1; then \
			$(LOG_ERROR) "python3 is required to install pre-commit"; \
			exit 1; \
		fi; \
		$(LOG_WARN) "pre-commit not found, installing with pip --user"; \
		$(PYTHON3) -m pip install --user $(PRE_COMMIT); \
		PRE_COMMIT_BIN="$$(command -v $(PRE_COMMIT) || true)"; \
		if [ -z "$$PRE_COMMIT_BIN" ]; then \
			PRE_COMMIT_BIN="$$( $(PYTHON3) -m site --user-base )/bin/$(PRE_COMMIT)"; \
		fi; \
	fi; \
	if [ ! -x "$$PRE_COMMIT_BIN" ]; then \
		$(LOG_ERROR) "pre-commit was installed but is not executable"; \
		exit 1; \
	fi; \
	"$$PRE_COMMIT_BIN" install --install-hooks --hook-type pre-commit --hook-type commit-msg; \
	$(LOG_SUCCESS) "pre-commit hooks installed successfully"

## hooks.verify: Verify pre-commit hooks are installed
hooks.verify:
	@$(LOG_INFO) "Verifying pre-commit hooks"
	@if [ ! -f "$(PRE_COMMIT_FILE)" ]; then \
		$(LOG_ERROR) "Missing $(PRE_COMMIT_FILE)"; \
		exit 1; \
	fi
	@if [ ! -x "$(ROOT_DIR)/.git/hooks/pre-commit" ]; then \
		$(LOG_ERROR) "Missing .git/hooks/pre-commit. Run 'make hooks.install'."; \
		exit 1; \
	fi
	@if [ ! -x "$(ROOT_DIR)/.git/hooks/commit-msg" ]; then \
		$(LOG_ERROR) "Missing .git/hooks/commit-msg. Run 'make hooks.install'."; \
		exit 1; \
	fi
	@$(LOG_SUCCESS) "pre-commit hooks are installed"

## hooks.run: Run pre-commit hooks on staged files
hooks.run:
	@$(LOG_INFO) "Running pre-commit hooks on staged files"
	@if [ ! -f "$(PRE_COMMIT_FILE)" ]; then \
		$(LOG_ERROR) "Missing $(PRE_COMMIT_FILE)"; \
		exit 1; \
	fi
	@$(resolve-pre-commit-bin); \
	"$$PRE_COMMIT_BIN" run

## hooks.run-all: Run pre-commit hooks on all files
hooks.run-all:
	@$(LOG_INFO) "Running pre-commit hooks on all files"
	@if [ ! -f "$(PRE_COMMIT_FILE)" ]; then \
		$(LOG_ERROR) "Missing $(PRE_COMMIT_FILE)"; \
		exit 1; \
	fi
	@$(resolve-pre-commit-bin); \
	"$$PRE_COMMIT_BIN" run --all-files

## hooks.clean: Uninstall pre-commit hooks
hooks.clean:
	@$(LOG_INFO) "Uninstalling pre-commit hooks"
	@$(resolve-pre-commit-bin); \
	"$$PRE_COMMIT_BIN" uninstall --hook-type pre-commit --hook-type commit-msg; \
	$(LOG_SUCCESS) "pre-commit hooks removed"
