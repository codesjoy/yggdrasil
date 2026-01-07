# ==============================================================================
# Makefile helper functions for golang
#

GO := go

ifeq ($(GOOS),windows)
	GO_OUT_EXT := .exe
endif

GOPATH := $(shell go env GOPATH)
ifeq ($(origin GOBIN), undefined)
	GOBIN := $(GOPATH)/bin
endif

COMMANDS ?= $(filter-out %.md, $(wildcard ${ROOT_DIR}/cmd/*))
BINS ?= $(foreach cmd,${COMMANDS},$(notdir ${cmd}))

ifeq (${COMMANDS},)
  $(error Could not determine COMMANDS, set ROOT_DIR or run in source dir)
endif
ifeq (${BINS},)
  $(error Could not determine BINS, set ROOT_DIR or run in source dir)
endif

LINT_FLAGS ?=

# find all go mod path
# returns an array contains mod path
GO_MODULES ?= $(shell cd $(ROOT_DIR) && find . -not \
	\( \( -path './output' -o -path './.git' -o -path '*/third_party/*' -o -path '*/vendor/*' \) -prune \) \
	-name 'go.mod' -print0 | xargs -0 -I {} dirname {})

GO_MODULE_TARGETS ?= $(subst /,+,$(patsubst ./%,%,$(GO_MODULES)))

# exclude tests
EXCLUDE_TESTS=

.PHONY: go.build.%
go.build.%:
	$(eval COMMAND := $(word 2,$(subst ., ,$*)))
	$(eval PLATFORM := $(word 1,$(subst ., ,$*)))
	$(eval OS := $(word 1,$(subst _, ,$(PLATFORM))))
	$(eval ARCH := $(word 2,$(subst _, ,$(PLATFORM))))
	@$(LOG_INFO) "Building binary $(COMMAND) $(VERSION) for $(OS) $(ARCH)"
	@mkdir -p $(OUTPUT_DIR)/platforms/$(OS)/$(ARCH)
	@CGO_ENABLED=0 GOOS=$(OS) GOARCH=$(ARCH) $(GO) build -o $(OUTPUT_DIR)/platforms/$(OS)/$(ARCH)/$(COMMAND)$(GO_OUT_EXT) $(ROOT_DIR)/cmd/$(COMMAND)

.PHONY: go.build
go.build: $(addprefix go.build., $(addprefix $(PLATFORM)., $(BINS)))

.PHONY: go.build.multiarch
go.build.multiarch: $(foreach p,$(PLATFORMS),$(addprefix go.build., $(addprefix $(p)., $(BINS))))

.PHONY: go.install.%
go.install.%:
	$(eval COMMAND := $(word 2,$(subst ., ,$*)))
	$(eval PLATFORM := $(word 1,$(subst ., ,$*)))
	$(eval OS := $(word 1,$(subst _, ,$(PLATFORM))))
	$(eval ARCH := $(word 2,$(subst _, ,$(PLATFORM))))
	@$(LOG_INFO) "Building binary $(COMMAND) $(VERSION) for $(OS) $(ARCH)"
	@mkdir -p $(OUTPUT_DIR)/platforms/$(OS)/$(ARCH)
	@CGO_ENABLED=0 GOOS=$(OS) GOARCH=$(ARCH) $(GO) install $(ROOT_DIR)/cmd/$(COMMAND)

.PHONY: go.install
go.install: $(addprefix go.install., $(addprefix $(PLATFORM)., $(BINS)))

.PHONY: go.lint.%
go.lint.%: tools.verify.golangci-lint
	$(eval MODULE_PATH := $(subst +,/,$*))
	@$(LOG_INFO) "Installing golangci-lint on module: $(MODULE_PATH)"
	@cd $(ROOT_DIR)/$(MODULE_PATH) && golangci-lint run -c $(ROOT_DIR)/.golangci.yaml $(LINT_FLAGS) ./...

.PHONY: go.lint
go.lint: $(addprefix go.lint., $(GO_MODULE_TARGETS))

.PHONY: go.fix.%
go.fix.%: tools.verify.golangci-lint
	$(eval MODULE_PATH := $(subst +,/,$*))
	@$(LOG_INFO) " Run golangci-lint --fix on module: $(MODULE_PATH)"
	@cd $(ROOT_DIR)/$(MODULE_PATH) && golangci-lint run --fix -c $(ROOT_DIR)/.golangci.yaml $(LINT_FLAGS) ./...

.PHONY: go.fix
go.fix: $(addprefix go.fix., $(GO_MODULE_TARGETS))

.PHONY: go.test.%
go.test.%: tools.verify.go-junit-report
	$(eval MODULE_PATH := $(subst +,/,$*))
	@$(LOG_INFO) "Run unit test on module: $(MODULE_PATH)"
	@mkdir -p $(OUTPUT_DIR)/reports/$(MODULE_PATH)
	@set -o pipefail; cd $(ROOT_DIR)/$(MODULE_PATH) && $(GO) test -race -cover \
    		-coverprofile=$(OUTPUT_DIR)/reports/$(MODULE_PATH)/coverage.out \
    		-timeout=10m -shuffle=on -short -v \
    		`go list ./... | egrep -v $(subst $(SPACE),'|',$(sort $(EXCLUDE_TESTS)))` 2>&1 | \
    		tee >(go-junit-report --set-exit-code > $(OUTPUT_DIR)/reports/$(MODULE_PATH)/report.xml)
	@sed -i '/mock_.*.go/d' $(OUTPUT_DIR)/reports/$(MODULE_PATH)/coverage.out # remove mock_.*.go files from test coverage
	@$(GO) tool cover -html=$(OUTPUT_DIR)/reports/$(MODULE_PATH)/coverage.out -o $(OUTPUT_DIR)/reports/$(MODULE_PATH)/coverage.html

.PHONY: go.test
go.test: $(addprefix go.test., example)

.PHONY: go.test.coverage.%
go.test.coverage.%: go.test.%
	$(eval MODULE_PATH := $(subst +,/,$*))
	@$(LOG_INFO) "Checking coverage of module: $(MODULE_PATH)"
	@cd $(ROOT_DIR)/$(MODULE_PATH) && $(GO)tool cover -func=./coverage.out | \
		awk -v target=$(COVERAGE) -f $(ROOT_DIR)/scripts/coverage.awk

.PHONY: go.test.coverage
go.test.coverage: $(addprefix go.test.coverage., $(GO_MODULE_TARGETS))

.PHONY: go.work.verify
go.work.verify:
	@$(LOG_INFO) "Verifying go.work"
	@scripts/gowork.sh

.PHONY: go.sync
go.work.sync: go.work.verify
	@$(LOG_INFO) "Run go work sync"
	@cd $(ROOT_DIR) && go work sync

.PHONY: go.tidy
go.tidy: go.work.verify
	@cd $(ROOT_DIR) && go mod tidy && go work sync

.PHONY: go.download
go.download: go.work.verify
	@$(LOG_INFO) "Run go mod download"
	@cd $(ROOT_DIR) && go mod download && go work sync