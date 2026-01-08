# ==============================================================================
# Makefile helper functions for copyright
#
#
.PHONY: copyright.verify
copyright.verify: tools.verify.addlicense
	@$(LOG_INFO) "Verifying the boilerplate headers for all files"
	@addlicense --check -f $(ROOT_DIR)/scripts/boilerplate.txt \
		-ignore "vendor/**" -ignore "third_party/**" -ignore "_output/**" -ignore "**/*.yaml" -ignore "**/.*/**" \
		$(ROOT_DIR)

.PHONY: copyright.add
copyright.add: tools.verify.addlicense
	@$(LOG_INFO) "Add the boilerplate headers for all files"
	@addlicense -v -f $(ROOT_DIR)/scripts/boilerplate.txt \
		-ignore "vendor/**" -ignore "third_party/**" -ignore "_output/**" -ignore "**/*.yaml" -ignore "**/.*/**" \
		$(ROOT_DIR) 2>&1 | grep -v -E "skipping|already has"  || true
