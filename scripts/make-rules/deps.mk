# ==============================================================================
# Dependency Management
# ==============================================================================
# Manages Go modules and workspace dependencies.
#
# Targets:
#   - go.tidy:          Run 'go mod tidy' on all modules
#   - go.mod.download:  Download dependencies for all modules
#   - go.mod.verify:    Verify dependencies integrity
#   - go.mod.tidy.check:Verify go.mod/go.sum tidy state without modifying files
#   - go.work.sync:     Sync go.work with all discovered go.mod modules
#   - go.work.drift:    Detect drift between go.work and discovered modules
#   - go.work.init:     Initialize go.work if not exists
#   - go.clean:         Remove build artifacts and caches
#
# Usage:
#   make tidy           - Tidy all modules
#   make sync           - Initialize/sync go workspace
#   make clean          - Clean all artifacts
# ==============================================================================

# ==============================================================================
# PHONY Targets
# ==============================================================================
.PHONY: go.tidy go.mod.download go.mod.verify \
        go.mod.tidy.check \
        go.work.verify go.work.sync go.work.init go.work.drift \
        go.clean

## go.tidy: Tidy go.mod for all modules
go.tidy:
	$(call validate-module-selection,$(ALL_MODULES))
	@$(LOG_INFO) "Tidying go.mod for all modules"
	@source $(ROOT_DIR)/scripts/lib/logger.sh >/dev/null 2>&1; \
	for module in $(ALL_MODULES); do \
		log::info "$$module"; \
		cd "$(ROOT_DIR)/$$module" && $(GO_IN_MODULE) mod tidy || exit 1; \
	done
	@$(LOG_SUCCESS) "All modules tidied"

## go.mod.download: Download dependencies for all modules
go.mod.download:
	$(call validate-module-selection,$(ALL_MODULES))
	@$(LOG_INFO) "Downloading dependencies"
	@source $(ROOT_DIR)/scripts/lib/logger.sh >/dev/null 2>&1; \
	for module in $(ALL_MODULES); do \
		log::info "$$module"; \
		cd "$(ROOT_DIR)/$$module" && $(GO_IN_MODULE) mod download || exit 1; \
	done
	@$(LOG_SUCCESS) "Dependencies downloaded"

## go.mod.verify: Verify dependencies
go.mod.verify:
	$(call validate-module-selection,$(ALL_MODULES))
	@$(LOG_INFO) "Verifying dependencies"
	@source $(ROOT_DIR)/scripts/lib/logger.sh >/dev/null 2>&1; \
	for module in $(ALL_MODULES); do \
		log::info "$$module"; \
		cd "$(ROOT_DIR)/$$module" && $(GO_IN_MODULE) mod verify || exit 1; \
	done
	@$(LOG_SUCCESS) "Dependencies verified"

## go.mod.tidy.check: Check go.mod/go.sum are tidy without modifying files
go.mod.tidy.check:
	$(call validate-module-selection,$(ALL_MODULES))
	@$(LOG_INFO) "Checking go.mod/go.sum tidy state"
	@source $(ROOT_DIR)/scripts/lib/logger.sh >/dev/null 2>&1; \
	for module in $(ALL_MODULES); do \
		log::info "$$module"; \
		cd "$(ROOT_DIR)/$$module" || exit 1; \
		tidy_diff=$$($(GO_IN_MODULE) mod tidy -diff 2>&1) || { \
			echo "$$tidy_diff"; \
			exit 1; \
		}; \
		if [ -n "$$tidy_diff" ]; then \
			$(LOG_ERROR) "$$module has go.mod/go.sum drift. Run 'make tidy'."; \
			echo "$$tidy_diff"; \
			exit 1; \
		fi; \
	done
	@$(LOG_SUCCESS) "All modules are tidy"

## go.work.verify: Verify go.work exists
go.work.verify:
	@$(LOG_INFO) "Verifying go.work"
	@if [ ! -f "$(ROOT_DIR)/go.work" ]; then \
		$(LOG_WARN) "go.work not found. Initializing workspace."; \
		$(MAKE) --no-print-directory go.work.init; \
	fi
	@if [ ! -f "$(ROOT_DIR)/go.work" ]; then \
		$(LOG_ERROR) "go.work not found after initialization."; \
		exit 1; \
	fi
	@$(LOG_SUCCESS) "go.work verified"

## go.work.sync: Sync go workspace with all modules
go.work.sync: go.work.init
	$(call validate-module-selection,$(ALL_MODULES))
	@$(LOG_INFO) "Syncing go workspace"
	@cd $(ROOT_DIR); \
	go_version=$$($(GO) version | awk '{print $$3}' | sed 's/go//'); \
	echo "go $$go_version" > go.work; \
	echo "" >> go.work; \
	echo "use (" >> go.work; \
	for module in $(ALL_MODULES); do \
		if [ "$$module" = "." ]; then \
			echo "	." >> go.work; \
		else \
			echo "	./$$module" >> go.work; \
		fi; \
	done; \
	echo ")" >> go.work
	@$(LOG_SUCCESS) "Go workspace synced"

## go.work.drift: Check whether go.work matches discovered modules
go.work.drift: go.work.verify
	$(call validate-module-selection,$(ALL_MODULES))
	@$(LOG_INFO) "Checking go.work drift"
	@expected_file=$$(mktemp); current_file=$$(mktemp); \
	trap 'rm -f "$$expected_file" "$$current_file"' EXIT; \
	printf '%s\n' $(ALL_MODULES) | sed -e 's|^\./||' -e 's|^$$|.|' | sort > "$$expected_file"; \
	awk '\
		/^use \(/ { in_use=1; next } \
		in_use && /^\)/ { exit } \
		in_use { gsub(/^[[:space:]]+|[[:space:]]+$$/, "", $$0); if ($$0 != "") print $$0 } \
	' "$(ROOT_DIR)/go.work" | sed -e 's|^\./||' -e 's|^$$|.|' | sort > "$$current_file"; \
	if ! diff -u "$$current_file" "$$expected_file" >/dev/null; then \
		$(LOG_ERROR) "go.work is out of sync with discovered modules. Run 'make sync'."; \
		diff -u "$$current_file" "$$expected_file" || true; \
		exit 1; \
	fi
	@$(LOG_SUCCESS) "go.work is in sync with discovered modules"

## go.work.init: Initialize go workspace
go.work.init:
	@if [ ! -f "$(ROOT_DIR)/go.work" ]; then \
		$(LOG_INFO) "Initializing go.work"; \
		cd $(ROOT_DIR) && bash scripts/gowork.sh; \
	fi

## go.clean: Clean build artifacts and caches
go.clean:
	$(call validate-module-selection,$(ALL_MODULES))
	@$(LOG_INFO) "Cleaning build artifacts"
	@$(RM) $(OUTPUT_DIR)
	@for module in $(ALL_MODULES); do \
		cd "$(ROOT_DIR)/$$module" && $(GO_IN_MODULE) clean -cache -testcache; \
	done
	@$(LOG_SUCCESS) "Cleaned build artifacts"
