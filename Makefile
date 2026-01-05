CODE_CHECK_TOOLS_SHELL="./scripts/code_check_tools.sh"
COPYRIGHT_SHELL="./scripts/copyright/update-copyright.sh"

# Makefile will execute chmod when execute makefile
$(shell chmod +x  ${CODE_CHECK_TOOLS_SHELL} ${COPYRIGHT_SHELL})
# Copy githook scripts when execute makefile
$(shell cp -f githooks/* .git/hooks/)

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