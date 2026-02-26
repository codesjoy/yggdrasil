# ==============================================================================
# Changelog
# ==============================================================================
# Generates changelog output from Conventional Commit history via git-chglog.
#
# Targets:
#   - changelog:              Generate and write CHANGELOG.md
#   - changelog.preview:      Preview changelog in stdout
#   - changelog.verify:       Verify CHANGELOG.md is up to date
#   - changelog.state.print:  Print changelog profile/state/query context
#   - changelog.state.reset:  Reset changelog baseline state to HEAD
# ==============================================================================

CHANGELOG_MANAGE         ?= $(ROOT_DIR)/scripts/changelog/manage.sh
CHANGELOG_FILE           ?= $(ROOT_DIR)/CHANGELOG.md
CHANGELOG_CONFIG         ?= $(ROOT_DIR)/.chglog/config.yml
CHANGELOG_TEMPLATE       ?= $(ROOT_DIR)/.chglog/CHANGELOG.tpl.md
CHANGELOG_QUERY          ?=
CHANGELOG_FROM           ?=
CHANGELOG_TO             ?=
CHANGELOG_NEXT_TAG       ?= unreleased
CHANGELOG_PATHS          ?=
CHANGELOG_SORT           ?= date
CHANGELOG_PROFILE        ?= balanced
CHANGELOG_CADENCE        ?= monthly
CHANGELOG_USE_BASELINE   ?= 1
CHANGELOG_ARCHIVE_ENABLE ?= 1
CHANGELOG_STATE_FILE     ?= $(ROOT_DIR)/.chglog/state.env
CHANGELOG_ARCHIVE_DIR    ?= $(ROOT_DIR)/.chglog/archive
CHANGELOG_NOW            ?=
CHANGELOG_STRICT_STATE   ?= 0

CHANGELOG_CADENCE_ORIGIN := $(origin CHANGELOG_CADENCE)
CHANGELOG_USE_BASELINE_ORIGIN := $(origin CHANGELOG_USE_BASELINE)
CHANGELOG_ARCHIVE_ENABLE_ORIGIN := $(origin CHANGELOG_ARCHIVE_ENABLE)

.PHONY: changelog changelog.preview changelog.verify \
        changelog.state.print changelog.state.reset

define run-changelog-manage
	@LOG_LEVEL="$(LOG_LEVEL)" \
	GIT_CHGLOG="$(GIT_CHGLOG)" \
	CHANGELOG_FILE="$(CHANGELOG_FILE)" \
	CHANGELOG_CONFIG="$(CHANGELOG_CONFIG)" \
	CHANGELOG_TEMPLATE="$(CHANGELOG_TEMPLATE)" \
	CHANGELOG_QUERY="$(CHANGELOG_QUERY)" \
	CHANGELOG_FROM="$(CHANGELOG_FROM)" \
	CHANGELOG_TO="$(CHANGELOG_TO)" \
	CHANGELOG_NEXT_TAG="$(CHANGELOG_NEXT_TAG)" \
	CHANGELOG_PATHS="$(CHANGELOG_PATHS)" \
	CHANGELOG_SORT="$(CHANGELOG_SORT)" \
	CHANGELOG_PROFILE="$(CHANGELOG_PROFILE)" \
	CHANGELOG_CADENCE="$(CHANGELOG_CADENCE)" \
	CHANGELOG_USE_BASELINE="$(CHANGELOG_USE_BASELINE)" \
	CHANGELOG_ARCHIVE_ENABLE="$(CHANGELOG_ARCHIVE_ENABLE)" \
	CHANGELOG_STATE_FILE="$(CHANGELOG_STATE_FILE)" \
	CHANGELOG_ARCHIVE_DIR="$(CHANGELOG_ARCHIVE_DIR)" \
	CHANGELOG_NOW="$(CHANGELOG_NOW)" \
	CHANGELOG_STRICT_STATE="$(CHANGELOG_STRICT_STATE)" \
	CHANGELOG_CADENCE_ORIGIN="$(CHANGELOG_CADENCE_ORIGIN)" \
	CHANGELOG_USE_BASELINE_ORIGIN="$(CHANGELOG_USE_BASELINE_ORIGIN)" \
	CHANGELOG_ARCHIVE_ENABLE_ORIGIN="$(CHANGELOG_ARCHIVE_ENABLE_ORIGIN)" \
	bash "$(CHANGELOG_MANAGE)" $(1)
endef

## changelog: Generate and write CHANGELOG.md
changelog:
	@$(call require-tool,$(GIT_CHGLOG))
	@$(call require-file,$(CHANGELOG_CONFIG))
	@$(call require-file,$(CHANGELOG_TEMPLATE))
	@$(call require-file,$(CHANGELOG_MANAGE))
	@$(call run-changelog-manage,generate)

## changelog.preview: Preview changelog content to stdout
changelog.preview:
	@$(call require-tool,$(GIT_CHGLOG))
	@$(call require-file,$(CHANGELOG_CONFIG))
	@$(call require-file,$(CHANGELOG_TEMPLATE))
	@$(call require-file,$(CHANGELOG_MANAGE))
	@$(call run-changelog-manage,preview)

## changelog.verify: Verify CHANGELOG.md matches generated content
changelog.verify:
	@$(call require-tool,$(GIT_CHGLOG))
	@$(call require-file,$(CHANGELOG_CONFIG))
	@$(call require-file,$(CHANGELOG_TEMPLATE))
	@$(call require-file,$(CHANGELOG_MANAGE))
	@$(call run-changelog-manage,verify)

## changelog.state.print: Print changelog profile/state and resolved query
changelog.state.print:
	@$(call require-file,$(CHANGELOG_MANAGE))
	@$(call run-changelog-manage,state-print)

## changelog.state.reset: Reset changelog baseline state to HEAD
changelog.state.reset:
	@$(call require-file,$(CHANGELOG_MANAGE))
	@$(call run-changelog-manage,state-reset)
