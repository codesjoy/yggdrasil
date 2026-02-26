# ==============================================================================
# Copyright Management
# ==============================================================================
# Manages copyright headers in Go and shell source files.
#
# Targets:
#   - copyright.verify:  Check if all files have correct copyright headers
#   - copyright.add:     Add/update copyright headers to all files
#   - copyright.check:   Alias for verify
#
# Configuration:
#   - BOILERPLATE:       Copyright template file (scripts/boilerplate.txt)
#   - COPYRIGHT_TOOL:    Tool to use (addlicense)
#   - COPYRIGHT_FILES:   Files to process (auto-discovered)
#
# Usage:
#   make copyright       - Verify copyright headers (CI gate)
#   make copyright.add   - Add missing copyright headers
#
# Files are automatically discovered from $(ROOT_DIR) excluding:
#   - vendor/
#   - _output/
#   - .tmp/
# ==============================================================================

BOILERPLATE := $(ROOT_DIR)/scripts/boilerplate.txt
COPYRIGHT_TOOL := addlicense

# Files to check for copyright (optimized with find's built-in exclusion)
COPYRIGHT_FILES := $(shell find $(ROOT_DIR) \( -name "*.go" -o -name "*.sh" \) \
    -not -path "*/vendor/*" -not -path "*/_output/*" -not -path "*/.tmp/*")

# ==============================================================================
# PHONY Targets
# ==============================================================================
.PHONY: copyright.verify copyright.add copyright.check

## copyright.verify: Verify copyright headers
copyright.verify:
	@$(LOG_INFO) "Verifying copyright headers"
	@if ! command -v $(COPYRIGHT_TOOL) >/dev/null 2>&1; then \
		$(LOG_ERROR) "$(COPYRIGHT_TOOL) not found. Run 'make tools' to install."; \
		exit 1; \
	fi
	@$(COPYRIGHT_TOOL) -check -f $(BOILERPLATE) $(COPYRIGHT_FILES) || \
		{ $(LOG_ERROR) "Copyright headers missing or incorrect. Run 'make copyright.add' to fix."; exit 1; }
	@$(LOG_SUCCESS) "All copyright headers verified"

## copyright.add: Add copyright headers to files
copyright.add:
	@$(LOG_INFO) "Adding copyright headers"
	@if ! command -v $(COPYRIGHT_TOOL) >/dev/null 2>&1; then \
		$(LOG_INFO) "Installing $(COPYRIGHT_TOOL)"; \
		$(GO) install github.com/google/addlicense@latest; \
	fi
	@$(COPYRIGHT_TOOL) -f $(BOILERPLATE) -y $(shell date +%Y) $(COPYRIGHT_FILES)
	@$(LOG_SUCCESS) "Copyright headers added"

## copyright.check: Check copyright (alias for verify)
copyright.check: copyright.verify
