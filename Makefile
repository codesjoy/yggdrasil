CODE_CHECK_TOOLS_SHELL="./scripts/code_check_tools.sh"
COPYRIGHT_SHELL="./scripts/copyright/update-copyright.sh"

.PHONY: lint
lint:
	@${CODE_CHECK_TOOLS_SHELL} lint "$(mod)"
	@echo "lint check finished"

.PHONY: fix
fix: $(LINTER)
	@${CODE_CHECK_TOOLS_SHELL} fix "$(mod)"
	@echo "lint fix finished"

.PHONY: copyright
copyright:
	@${COPYRIGHT_SHELL}
	@echo "add copyright finish"