# ==============================================================================
# Makefile helper functions for tools
#

TOOLS ?=$(BLOCKER_TOOLS) $(CRITICAL_TOOLS) $(TRIVIAL_TOOLS)

.PHONY: tools.install
tools.install: $(addprefix tools.install., $(TOOLS))

.PHONY: tools.install.%
tools.install.%:
	@$(MAKE) install.$*

.PHONY: tools.verify.%
tools.verify.%:
	@$(LOG_INFO) "Verifying had be installed for tool $*"
	@if ! which $* &>/dev/null; then $(MAKE) tools.install.$*; fi

.PHONY: install.golangci-lint
install.golangci-lint:
	@$(LOG_INFO) "Installing golangci-lint"
	@$(GO) install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.7.2
	@golangci-lint completion bash > $(HOME)/.golangci-lint.bash
	@if ! grep -q .golangci-lint.bash $(HOME)/.bashrc; then echo "source \$$HOME/.golangci-lint.bash" >> $(HOME)/.bashrc; fi

.PHONY: install.git-chglog
install.git-chglog:
	@$(LOG_INFO) "Installing git-chglog"
	@$(GO) install github.com/git-chglog/git-chglog/cmd/git-chglog@latest

.PHONY: install.addlicense
install.addlicense:
	@$(LOG_INFO) "Installing addlicense"
	@$(GO) install github.com/google/addlicense@latest

.PHONY: install.go-junit-report
install.go-junit-report:
	@$(LOG_INFO) "Installing go-junit-report"
	@$(GO) install github.com/jstemmer/go-junit-report/v2@latest

.PHONY: install.buf
install.buf:
	@$(LOG_INFO) "Installing buf"
	@$(GO) install github.com/bufbuild/buf/cmd/buf@v1.63.0
