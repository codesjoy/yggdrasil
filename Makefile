.PHONY: all
all:

# ==============================================================================
# Constant definition

# ==============================================================================

# ==============================================================================
# Includes

include scripts/make-rules/common.mk # make sure include common.mk at the first include line
include scripts/make-rules/copyright.mk
include scripts/make-rules/golang.mk
include scripts/make-rules/tools.mk

# ==============================================================================

define USAGE_OPTIONS

Options:
  DEBUG            Whether to generate debug symbols. Default is 0.
  BINS             The binaries to build. Default is all of cmd.
                   This option is available when using: make build/build.multiarch
                   Example: make build BINS="cmd-one cmd-two"
  PLATFORMS        The multiple platforms to build. Default is linux_amd64 and linux_arm64.
                   This option is available when using: make build.multiarch/image.multiarch/push.multiarch
                   Example: make image.multiarch IMAGES="iam-apiserver iam-pump" PLATFORMS="linux_amd64 linux_arm64"
  V                Set to 1 enable verbose build. Default is 0.
endef
export USAGE_OPTIONS

## help: Show help information.
.PHONY: help
help: Makefile
	@printf "\nUsage: make <TARGETS> <OPTIONS> ...\n\nTargets:\n"
	@sed -n 's/^##//p' $< | column -t -s ':' | sed -e 's/^/ /'
	@echo "$$USAGE_OPTIONS"

.PHONY: fix-permissions
fix-permissions:
	@find $(ROOT_DIR)/scripts -type f -name "*.sh" -exec chmod +x {} +
	@find $(ROOT_DIR)/scripts -type f -name "*.sh" -exec dos2unix {} +

## tools: Install the relevant tools.
.PHONY: tools
tools:
	@$(LOG_INFO) "install tools"
	@$(MAKE) tools.install -j$(nproc)

.PHONY: clean
clean:
	@$(LOG_INFO) "Cleaning output"
	@-rm -rf $(OUTPUT_DIR)

## copyright: add copyright header.
.PHONY: copyright
copyright:
	@$(LOG_INFO) "add copyright header"
	@$(MAKE) copyright.add -j$(nproc)

## build: Build the binaries.
.PHONY: build
build:
	@$(MAKE) go.build -j$(nproc)

## build: Install the binaries.
.PHONY: install
install:
	@$(MAKE) go.install -j$(nproc)

## lint: Lint the code.
.PHONY: lint
lint:
	@$(MAKE) go.lint

## fix: Fix the code.
.PHONY: fix
fix:
	@$(MAKE) go.fix

## test: Run the tests.
.PHONY: test
test:
	@$(MAKE) go.test -j$(nproc)

## coverage: Generate the coverage report.
.PHONY: coverage
coverage:
	@$(MAKE) go.test.coverage -j$(nproc)

## sync: Sync the code.
.PHONY: sync
sync:
	@$(MAKE) go.work.sync

## tidy: Tidy the code.
.PHONY: tidy
tidy:
	@$(MAKE) go.tidy -j$(nproc)

## download: Download the dependencies.
.PHONY: download
download:
	@$(MAKE) go.download -j$(nproc)


