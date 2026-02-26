#!/usr/bin/env bash
# Copyright 2022 The codesjoy Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -o errexit
set -o nounset
set -o pipefail

source "$(dirname "${BASH_SOURCE[0]}")/../lib/init.sh"
set -o nounset

ACTION="${1:-}"

GIT_CHGLOG_BIN="${GIT_CHGLOG:-git-chglog}"
CHANGELOG_FILE="${CHANGELOG_FILE:-${ROOT_DIR}/CHANGELOG.md}"
CHANGELOG_CONFIG="${CHANGELOG_CONFIG:-${ROOT_DIR}/.chglog/config.yml}"
CHANGELOG_TEMPLATE="${CHANGELOG_TEMPLATE:-${ROOT_DIR}/.chglog/CHANGELOG.tpl.md}"
CHANGELOG_QUERY="${CHANGELOG_QUERY:-}"
CHANGELOG_FROM="${CHANGELOG_FROM:-}"
CHANGELOG_TO="${CHANGELOG_TO:-}"
CHANGELOG_NEXT_TAG="${CHANGELOG_NEXT_TAG:-unreleased}"
CHANGELOG_PATHS="${CHANGELOG_PATHS:-}"
CHANGELOG_SORT="${CHANGELOG_SORT:-date}"
CHANGELOG_PROFILE="${CHANGELOG_PROFILE:-balanced}"
CHANGELOG_CADENCE="${CHANGELOG_CADENCE:-monthly}"
CHANGELOG_USE_BASELINE="${CHANGELOG_USE_BASELINE:-1}"
CHANGELOG_ARCHIVE_ENABLE="${CHANGELOG_ARCHIVE_ENABLE:-1}"
CHANGELOG_STATE_FILE="${CHANGELOG_STATE_FILE:-${ROOT_DIR}/.chglog/state.env}"
CHANGELOG_ARCHIVE_DIR="${CHANGELOG_ARCHIVE_DIR:-${ROOT_DIR}/.chglog/archive}"
CHANGELOG_NOW="${CHANGELOG_NOW:-}"
CHANGELOG_STRICT_STATE="${CHANGELOG_STRICT_STATE:-0}"

CHANGELOG_CADENCE_ORIGIN="${CHANGELOG_CADENCE_ORIGIN:-file}"
CHANGELOG_USE_BASELINE_ORIGIN="${CHANGELOG_USE_BASELINE_ORIGIN:-file}"
CHANGELOG_ARCHIVE_ENABLE_ORIGIN="${CHANGELOG_ARCHIVE_ENABLE_ORIGIN:-file}"

BASE_SHA=""
LAST_SHA=""
CURRENT_BUCKET=""

PATH_ARGS=()
TEMP_FILES=()
TMP_TAG_NAMES=()
TMP_TAG_SHAS=()
TEMP_TAG_RESULT=""
QUERY_RESULT=""

trim() {
	local s="${1}"
	s="${s#"${s%%[![:space:]]*}"}"
	s="${s%"${s##*[![:space:]]}"}"
	printf '%s' "${s}"
}

is_explicit_origin() {
	case "${1}" in
	"command line" | "environment" | "environment override" | "override")
		return 0
		;;
	*)
		return 1
		;;
	esac
}

normalize_toggle() {
	local name="${1}"
	local value="${2}"
	if [[ "${value}" != "0" && "${value}" != "1" ]]; then
		log::error "${name} must be 0 or 1, got: ${value}"
		exit 1
	fi
}

apply_profile_defaults() {
	local profile_cadence=""
	local profile_use_baseline=""
	local profile_archive_enable=""

	case "${CHANGELOG_PROFILE}" in
	simple)
		profile_cadence="none"
		profile_use_baseline="0"
		profile_archive_enable="0"
		;;
	balanced)
		profile_cadence="monthly"
		profile_use_baseline="1"
		profile_archive_enable="1"
		;;
	high-frequency)
		profile_cadence="weekly"
		profile_use_baseline="1"
		profile_archive_enable="1"
		;;
	*)
		log::error "Unsupported CHANGELOG_PROFILE: ${CHANGELOG_PROFILE} (allowed: simple, balanced, high-frequency)"
		exit 1
		;;
	esac

	if ! is_explicit_origin "${CHANGELOG_CADENCE_ORIGIN}"; then
		CHANGELOG_CADENCE="${profile_cadence}"
	fi
	if ! is_explicit_origin "${CHANGELOG_USE_BASELINE_ORIGIN}"; then
		CHANGELOG_USE_BASELINE="${profile_use_baseline}"
	fi
	if ! is_explicit_origin "${CHANGELOG_ARCHIVE_ENABLE_ORIGIN}"; then
		CHANGELOG_ARCHIVE_ENABLE="${profile_archive_enable}"
	fi

	case "${CHANGELOG_CADENCE}" in
	monthly | weekly | none) ;;
	*)
		log::error "Unsupported CHANGELOG_CADENCE: ${CHANGELOG_CADENCE} (allowed: monthly, weekly, none)"
		exit 1
		;;
	esac

	normalize_toggle "CHANGELOG_USE_BASELINE" "${CHANGELOG_USE_BASELINE}"
	normalize_toggle "CHANGELOG_ARCHIVE_ENABLE" "${CHANGELOG_ARCHIVE_ENABLE}"
	normalize_toggle "CHANGELOG_STRICT_STATE" "${CHANGELOG_STRICT_STATE}"
}

cleanup_temp_artifacts() {
	local f=""
	local tag=""
	if ((${#TEMP_FILES[@]} > 0)); then
		for f in "${TEMP_FILES[@]}"; do
			rm -f "${f}" >/dev/null 2>&1 || true
		done
	fi
	if ((${#TMP_TAG_NAMES[@]} > 0)); then
		for tag in "${TMP_TAG_NAMES[@]}"; do
			git tag -d "${tag}" >/dev/null 2>&1 || true
		done
	fi
}
trap cleanup_temp_artifacts EXIT

register_temp_file() {
	TEMP_FILES+=("${1}")
}

register_temp_tag() {
	TMP_TAG_NAMES+=("${1}")
	TMP_TAG_SHAS+=("${2}")
}

ensure_render_prereqs() {
	if ! command -v "${GIT_CHGLOG_BIN}" >/dev/null 2>&1; then
		log::error "Required tool '${GIT_CHGLOG_BIN}' not found. Run 'make tools'."
		exit 1
	fi
	if [[ ! -f "${CHANGELOG_CONFIG}" ]]; then
		log::error "Required file '${CHANGELOG_CONFIG}' not found"
		exit 1
	fi
	if [[ ! -f "${CHANGELOG_TEMPLATE}" ]]; then
		log::error "Required file '${CHANGELOG_TEMPLATE}' not found"
		exit 1
	fi
}

parse_paths() {
	PATH_ARGS=()
	if [[ -z "${CHANGELOG_PATHS}" ]]; then
		return 0
	fi

	local path_list=()
	IFS=' ' read -r -a path_list <<<"${CHANGELOG_PATHS}"
	local p=""
	for p in "${path_list[@]}"; do
		if [[ -n "${p}" ]]; then
			PATH_ARGS+=(--path "${p}")
		fi
	done
}

repo_has_tags() {
	[[ -n "$(git tag --list | head -n 1)" ]]
}

resolve_manual_query() {
	if [[ -n "${CHANGELOG_QUERY}" ]]; then
		printf '%s' "${CHANGELOG_QUERY}"
		return 0
	fi

	if [[ -n "${CHANGELOG_FROM}" && -n "${CHANGELOG_TO}" ]]; then
		printf '%s..%s' "${CHANGELOG_FROM}" "${CHANGELOG_TO}"
		return 0
	fi
	if [[ -n "${CHANGELOG_FROM}" ]]; then
		printf '%s..' "${CHANGELOG_FROM}"
		return 0
	fi
	if [[ -n "${CHANGELOG_TO}" ]]; then
		printf '..%s' "${CHANGELOG_TO}"
		return 0
	fi

	printf ''
}

create_temp_tag_for_ref() {
	local ref="${1}"
	local sha=""
	local tag=""
	sha="$(git rev-parse --verify "${ref}^{commit}" 2>/dev/null)" || {
		log::error "Unable to resolve ref as commit: ${ref}"
		exit 1
	}
	tag="__chglog_tmp_${sha}"
	git tag -f "${tag}" "${sha}" >/dev/null
	register_temp_tag "${tag}" "${sha}"
	TEMP_TAG_RESULT="${tag}"
}

replace_tmp_tags_with_shas() {
	local file_path="${1}"
	local i=0
	local old=""
	local new=""
	if ((${#TMP_TAG_NAMES[@]} == 0)); then
		return 0
	fi
	for ((i = 0; i < ${#TMP_TAG_NAMES[@]}; i++)); do
		old="${TMP_TAG_NAMES[$i]}"
		new="${TMP_TAG_SHAS[$i]}"
		python3 - "${file_path}" "${old}" "${new}" <<'PY'
import pathlib
import sys

file_path = pathlib.Path(sys.argv[1])
old = sys.argv[2]
new = sys.argv[3]
text = file_path.read_text()
text = text.replace(old, new)
file_path.write_text(text)
PY
	done
}

compute_bucket() {
	local cadence="${1}"
	if [[ "${cadence}" == "none" ]]; then
		printf 'none'
		return 0
	fi

	CHANGELOG_NOW_INPUT="${CHANGELOG_NOW}" CHANGELOG_CADENCE_INPUT="${cadence}" python3 - <<'PY'
import datetime
import os
import sys

cadence = os.environ.get("CHANGELOG_CADENCE_INPUT", "")
raw_now = os.environ.get("CHANGELOG_NOW_INPUT", "").strip()

if raw_now:
    candidates = [
        "%Y-%m-%d",
        "%Y-%m-%d %H:%M:%S",
        "%Y-%m-%dT%H:%M:%S",
        "%Y-%m-%dT%H:%M:%SZ",
    ]
    dt = None
    for fmt in candidates:
        try:
            dt = datetime.datetime.strptime(raw_now, fmt)
            break
        except ValueError:
            pass
    if dt is None:
        try:
            dt = datetime.datetime.fromisoformat(raw_now)
        except ValueError:
            sys.stderr.write(f"invalid CHANGELOG_NOW: {raw_now}\n")
            sys.exit(1)
else:
    dt = datetime.datetime.now()

if cadence == "monthly":
    print(dt.strftime("%Y-%m"))
elif cadence == "weekly":
    iso_year, iso_week, _ = dt.isocalendar()
    print(f"{iso_year}-W{iso_week:02d}")
else:
    sys.stderr.write(f"unsupported cadence: {cadence}\n")
    sys.exit(1)
PY
}

compute_current_date() {
	CHANGELOG_NOW_INPUT="${CHANGELOG_NOW}" python3 - <<'PY'
import datetime
import os
import sys

raw_now = os.environ.get("CHANGELOG_NOW_INPUT", "").strip()
if raw_now:
    candidates = [
        "%Y-%m-%d",
        "%Y-%m-%d %H:%M:%S",
        "%Y-%m-%dT%H:%M:%S",
        "%Y-%m-%dT%H:%M:%SZ",
    ]
    dt = None
    for fmt in candidates:
        try:
            dt = datetime.datetime.strptime(raw_now, fmt)
            break
        except ValueError:
            pass
    if dt is None:
        try:
            dt = datetime.datetime.fromisoformat(raw_now)
        except ValueError:
            sys.stderr.write(f"invalid CHANGELOG_NOW: {raw_now}\n")
            sys.exit(1)
else:
    dt = datetime.datetime.now()

print(dt.strftime("%Y-%m-%d"))
PY
}

load_state() {
	BASE_SHA=""
	LAST_SHA=""
	CURRENT_BUCKET=""

	if [[ ! -f "${CHANGELOG_STATE_FILE}" ]]; then
		return 0
	fi

	local malformed="0"
	local raw_line=""
	local line=""
	while IFS= read -r raw_line || [[ -n "${raw_line}" ]]; do
		line="$(trim "${raw_line}")"
		if [[ -z "${line}" || "${line}" == \#* ]]; then
			continue
		fi
		case "${line}" in
		BASE_SHA=*)
			BASE_SHA="${line#BASE_SHA=}"
			;;
		LAST_SHA=*)
			LAST_SHA="${line#LAST_SHA=}"
			;;
		CURRENT_BUCKET=*)
			CURRENT_BUCKET="${line#CURRENT_BUCKET=}"
			;;
		*)
			malformed="1"
			;;
		esac
	done <"${CHANGELOG_STATE_FILE}"

	if [[ -n "${BASE_SHA}" ]] && ! git rev-parse --verify "${BASE_SHA}^{commit}" >/dev/null 2>&1; then
		malformed="1"
	fi
	if [[ -n "${LAST_SHA}" ]] && ! git rev-parse --verify "${LAST_SHA}^{commit}" >/dev/null 2>&1; then
		malformed="1"
	fi
	if [[ -n "${CURRENT_BUCKET}" ]] && [[ ! "${CURRENT_BUCKET}" =~ ^([0-9]{4}-[0-9]{2}|[0-9]{4}-W[0-9]{2}|none)$ ]]; then
		malformed="1"
	fi

	if [[ "${malformed}" == "1" ]]; then
		if [[ "${CHANGELOG_STRICT_STATE}" == "1" ]]; then
			log::error "State file is malformed: ${CHANGELOG_STATE_FILE}. Run 'make changelog.state.reset'."
			exit 1
		fi
		log::warn "State file malformed, auto-resetting: ${CHANGELOG_STATE_FILE}"
		BASE_SHA=""
		LAST_SHA=""
		CURRENT_BUCKET=""
	fi
}

write_state() {
	local state_dir=""
	state_dir="$(dirname "${CHANGELOG_STATE_FILE}")"
	mkdir -p "${state_dir}"

	local tmp_file=""
	tmp_file="$(mktemp "${CHANGELOG_STATE_FILE}.tmp.XXXXXX")"
	register_temp_file "${tmp_file}"

	{
		echo "BASE_SHA=${BASE_SHA}"
		echo "LAST_SHA=${LAST_SHA}"
		echo "CURRENT_BUCKET=${CURRENT_BUCKET}"
	} >"${tmp_file}"

	mv "${tmp_file}" "${CHANGELOG_STATE_FILE}"
}

run_chglog() {
	local output_file="${1}"
	local query="${2}"
	local next_tag="${3}"

	local cmd=()
	cmd=("${GIT_CHGLOG_BIN}" --config "${CHANGELOG_CONFIG}" --template "${CHANGELOG_TEMPLATE}" --sort "${CHANGELOG_SORT}")
	if [[ -n "${output_file}" ]]; then
		cmd+=(--output "${output_file}")
	fi
	if [[ -n "${next_tag}" ]]; then
		cmd+=(--next-tag "${next_tag}")
	fi
	if ((${#PATH_ARGS[@]} > 0)); then
		cmd+=("${PATH_ARGS[@]}")
	fi
	if [[ -n "${query}" ]]; then
		cmd+=("${query}")
	fi

	"${cmd[@]}"
}

extract_first_version_section() {
	local input_file="${1}"
	local output_file="${2}"
	awk '
		/^<a name="/ { count++; if (count == 2) { exit } }
		count >= 1 { print }
	' "${input_file}" >"${output_file}"
}

relabel_section() {
	local file_path="${1}"
	local old_label="${2}"
	local new_label="${3}"
	python3 - "${file_path}" "${old_label}" "${new_label}" <<'PY'
import pathlib
import sys

file_path = pathlib.Path(sys.argv[1])
old = sys.argv[2]
new = sys.argv[3]
text = file_path.read_text()
text = text.replace(f'<a name="{old}"></a>', f'<a name="{new}"></a>', 1)
text = text.replace(f'## [{old}](', f'## [{new}](', 1)
text = text.replace(f'## {old} (', f'## {new} (', 1)
file_path.write_text(text)
PY
}

render_unreleased_section() {
	local output_file="${1}"
	local baseline_sha="${2}"

	if [[ "${CHANGELOG_USE_BASELINE}" != "1" || -z "${baseline_sha}" ]]; then
		run_chglog "${output_file}" "" "${CHANGELOG_NEXT_TAG}"
		return 0
	fi

	local from_tag=""
	local raw_output=""
	local first_section=""
	local baseline_resolved_sha=""
	local head_sha=""
	local today=""
	baseline_resolved_sha="$(git rev-parse --verify "${baseline_sha}^{commit}" 2>/dev/null)" || {
		log::error "Invalid baseline SHA in state: ${baseline_sha}"
		exit 1
	}
	head_sha="$(git rev-parse HEAD)"
	if [[ "${baseline_resolved_sha}" == "${head_sha}" ]]; then
		today="$(compute_current_date)"
		printf '<a name="%s"></a>\n## %s (%s)\n' "${CHANGELOG_NEXT_TAG}" "${CHANGELOG_NEXT_TAG}" "${today}" >"${output_file}"
		return 0
	fi

	create_temp_tag_for_ref "${baseline_resolved_sha}"
	from_tag="${TEMP_TAG_RESULT}"
	raw_output="$(mktemp)"
	first_section="$(mktemp)"
	register_temp_file "${raw_output}"
	register_temp_file "${first_section}"

	run_chglog "${raw_output}" "${from_tag}.." "${CHANGELOG_NEXT_TAG}"
	extract_first_version_section "${raw_output}" "${first_section}"
	replace_tmp_tags_with_shas "${first_section}"
	mv "${first_section}" "${output_file}"
}

render_archive_section() {
	local output_file="${1}"
	local bucket_label="${2}"
	local from_sha="${3}"
	local to_sha="${4}"

	if [[ -z "${to_sha}" ]]; then
		: >"${output_file}"
		return 0
	fi

	local from_ref="${from_sha}"
	if [[ -z "${from_ref}" ]]; then
		from_ref="$(git rev-list --max-parents=0 "${to_sha}" | tail -n 1)"
	fi

	local from_tag=""
	local to_tag=""
	local raw_output=""
	local first_section=""
	create_temp_tag_for_ref "${from_ref}"
	from_tag="${TEMP_TAG_RESULT}"
	create_temp_tag_for_ref "${to_sha}"
	to_tag="${TEMP_TAG_RESULT}"
	raw_output="$(mktemp)"
	first_section="$(mktemp)"
	register_temp_file "${raw_output}"
	register_temp_file "${first_section}"

	run_chglog "${raw_output}" "${from_tag}..${to_tag}" ""
	extract_first_version_section "${raw_output}" "${first_section}"
	relabel_section "${first_section}" "${to_tag}" "${bucket_label}"
	replace_tmp_tags_with_shas "${first_section}"
	mv "${first_section}" "${output_file}"
}

compose_changelog() {
	local current_file="${1}"
	local output_file="${2}"
	local include_archives="${3}"
	local prepend_archive_file="${4}"
	local skip_archive_basename="${5}"

	cat "${current_file}" >"${output_file}"

	if [[ "${include_archives}" != "1" ]]; then
		return 0
	fi

	if [[ -n "${prepend_archive_file}" && -f "${prepend_archive_file}" ]]; then
		printf '\n' >>"${output_file}"
		cat "${prepend_archive_file}" >>"${output_file}"
	fi

	if [[ ! -d "${CHANGELOG_ARCHIVE_DIR}" ]]; then
		return 0
	fi

	local archive_file=""
	while IFS= read -r archive_file; do
		if [[ -n "${skip_archive_basename}" && "$(basename "${archive_file}")" == "${skip_archive_basename}" ]]; then
			continue
		fi
		printf '\n' >>"${output_file}"
		cat "${archive_file}" >>"${output_file}"
	done < <(find "${CHANGELOG_ARCHIVE_DIR}" -maxdepth 1 -type f -name '*.md' | sort -r)
}

resolve_no_tag_query_fallback() {
	local query="${1}"
	QUERY_RESULT=""
	if [[ "${query}" == *".."* ]]; then
		local left_ref="${query%%..*}"
		local right_ref="${query#*..}"

		if [[ -z "${left_ref}" ]]; then
			local right_for_root="${right_ref}"
			if [[ -z "${right_for_root}" ]]; then
				right_for_root="HEAD"
			fi
			local right_sha=""
			right_sha="$(git rev-parse --verify "${right_for_root}^{commit}" 2>/dev/null)" || {
				log::error "Unable to resolve right boundary ref: ${right_for_root}"
				exit 1
			}
			left_ref="$(git rev-list --max-parents=0 "${right_sha}" | tail -n 1)"
		fi
		if [[ -z "${right_ref}" ]]; then
			right_ref="HEAD"
		fi

		local left_tag=""
		local right_tag=""
		create_temp_tag_for_ref "${left_ref}"
		left_tag="${TEMP_TAG_RESULT}"
		create_temp_tag_for_ref "${right_ref}"
		right_tag="${TEMP_TAG_RESULT}"
		QUERY_RESULT="${left_tag}..${right_tag}"
		return 0
	fi

	create_temp_tag_for_ref "${query}"
	QUERY_RESULT="${TEMP_TAG_RESULT}"
}

render_explicit_query() {
	local output_file="${1}"
	local query="${2}"
	local err_file=""
	err_file="$(mktemp)"
	register_temp_file "${err_file}"

	if run_chglog "${output_file}" "${query}" "" 2>"${err_file}"; then
		return 0
	fi

	if repo_has_tags; then
		if [[ -s "${err_file}" ]]; then
			cat "${err_file}" >&2
		fi
		log::error "Failed to generate changelog with query: ${query}"
		exit 1
	fi

	log::warn "No tags found, applying commit-based query fallback"
	local fallback_query=""
	resolve_no_tag_query_fallback "${query}"
	fallback_query="${QUERY_RESULT}"
	run_chglog "${output_file}" "${fallback_query}" ""
	replace_tmp_tags_with_shas "${output_file}"
}

render_managed() {
	local output_file="${1}"
	local update_state="${2}"

	load_state

	local head_sha=""
	head_sha="$(git rev-parse HEAD)"
	local current_bucket_now=""
	current_bucket_now="$(compute_bucket "${CHANGELOG_CADENCE}")"

	local base_for_current="${BASE_SHA}"
	local prepend_archive_file=""
	local skip_archive_basename=""

	if [[ "${CHANGELOG_ARCHIVE_ENABLE}" == "1" && "${CHANGELOG_CADENCE}" != "none" && -n "${CURRENT_BUCKET}" && "${CURRENT_BUCKET}" != "${current_bucket_now}" ]]; then
		if [[ -n "${LAST_SHA}" ]]; then
			local archive_target=""
			if [[ "${update_state}" == "1" ]]; then
				mkdir -p "${CHANGELOG_ARCHIVE_DIR}"
				archive_target="${CHANGELOG_ARCHIVE_DIR}/${CURRENT_BUCKET}.md"
				log::info "Archiving bucket ${CURRENT_BUCKET} -> ${archive_target}"
			else
				archive_target="$(mktemp)"
				register_temp_file "${archive_target}"
				prepend_archive_file="${archive_target}"
				skip_archive_basename="${CURRENT_BUCKET}.md"
				log::info "Simulating archive rollover for bucket ${CURRENT_BUCKET}"
			fi
			render_archive_section "${archive_target}" "${CURRENT_BUCKET}" "${BASE_SHA}" "${LAST_SHA}"
			if [[ "${CHANGELOG_USE_BASELINE}" == "1" ]]; then
				base_for_current="${LAST_SHA}"
			fi
		else
			log::warn "Bucket changed (${CURRENT_BUCKET} -> ${current_bucket_now}) but LAST_SHA is empty; skipping archive creation"
		fi
	fi

	local current_section_file=""
	current_section_file="$(mktemp)"
	register_temp_file "${current_section_file}"
	render_unreleased_section "${current_section_file}" "${base_for_current}"
	compose_changelog "${current_section_file}" "${output_file}" "${CHANGELOG_ARCHIVE_ENABLE}" "${prepend_archive_file}" "${skip_archive_basename}"

	if [[ "${update_state}" == "1" ]]; then
		if [[ "${CHANGELOG_USE_BASELINE}" == "1" && -n "${LAST_SHA}" && -n "${CURRENT_BUCKET}" && "${CURRENT_BUCKET}" != "${current_bucket_now}" ]]; then
			BASE_SHA="${LAST_SHA}"
		fi
		if [[ "${CHANGELOG_USE_BASELINE}" != "1" ]]; then
			BASE_SHA=""
		fi
		LAST_SHA="${head_sha}"
		CURRENT_BUCKET="${current_bucket_now}"
		write_state
	fi
}

state_print() {
	apply_profile_defaults
	load_state

	local current_bucket_now=""
	current_bucket_now="$(compute_bucket "${CHANGELOG_CADENCE}")"
	local manual_query=""
	manual_query="$(resolve_manual_query)"

	local resolved_query=""
	local query_source=""
	if [[ -n "${manual_query}" ]]; then
		resolved_query="${manual_query}"
		query_source="manual"
	elif [[ "${CHANGELOG_USE_BASELINE}" == "1" && -n "${BASE_SHA}" ]]; then
		resolved_query="${BASE_SHA}..HEAD"
		query_source="managed-baseline"
	else
		resolved_query="<full-history>"
		query_source="managed-full"
	fi

	echo "Changelog state:"
	echo "  profile:             ${CHANGELOG_PROFILE}"
	echo "  cadence:             ${CHANGELOG_CADENCE}"
	echo "  use_baseline:        ${CHANGELOG_USE_BASELINE}"
	echo "  archive_enable:      ${CHANGELOG_ARCHIVE_ENABLE}"
	echo "  current_bucket(now): ${current_bucket_now}"
	echo "  state_file:          ${CHANGELOG_STATE_FILE}"
	echo "  archive_dir:         ${CHANGELOG_ARCHIVE_DIR}"
	echo "  base_sha:            ${BASE_SHA:-<empty>}"
	echo "  last_sha:            ${LAST_SHA:-<empty>}"
	echo "  current_bucket:      ${CURRENT_BUCKET:-<empty>}"
	echo "  query_source:        ${query_source}"
	echo "  resolved_query:      ${resolved_query}"
}

state_reset() {
	apply_profile_defaults
	local head_sha=""
	head_sha="$(git rev-parse HEAD)"
	local current_bucket_now=""
	current_bucket_now="$(compute_bucket "${CHANGELOG_CADENCE}")"

	BASE_SHA="${head_sha}"
	LAST_SHA="${head_sha}"
	CURRENT_BUCKET="${current_bucket_now}"
	write_state
	log::success "Changelog state reset to HEAD (${head_sha})"
}

generate_changelog() {
	apply_profile_defaults
	ensure_render_prereqs
	parse_paths

	local manual_query=""
	manual_query="$(resolve_manual_query)"

	if [[ -n "${manual_query}" ]]; then
		log::info "Generating changelog in manual query mode: ${manual_query}"
		render_explicit_query "${CHANGELOG_FILE}" "${manual_query}"
		log::success "Changelog generated: ${CHANGELOG_FILE}"
		return 0
	fi

	log::info "Generating managed changelog: ${CHANGELOG_FILE}"
	render_managed "${CHANGELOG_FILE}" "1"
	log::success "Changelog generated: ${CHANGELOG_FILE}"
}

preview_changelog() {
	apply_profile_defaults
	ensure_render_prereqs
	parse_paths

	local manual_query=""
	manual_query="$(resolve_manual_query)"
	local tmp_output=""
	tmp_output="$(mktemp)"
	register_temp_file "${tmp_output}"

	if [[ -n "${manual_query}" ]]; then
		log::info "Previewing changelog in manual query mode: ${manual_query}"
		render_explicit_query "${tmp_output}" "${manual_query}"
	else
		log::info "Previewing managed changelog"
		render_managed "${tmp_output}" "0"
	fi

	cat "${tmp_output}"
}

verify_changelog() {
	apply_profile_defaults
	ensure_render_prereqs
	parse_paths

	if [[ ! -f "${CHANGELOG_FILE}" ]]; then
		log::error "${CHANGELOG_FILE} not found. Run 'make changelog' first."
		exit 1
	fi

	local manual_query=""
	manual_query="$(resolve_manual_query)"
	local tmp_output=""
	tmp_output="$(mktemp)"
	register_temp_file "${tmp_output}"

	if [[ -n "${manual_query}" ]]; then
		log::info "Verifying changelog in manual query mode: ${manual_query}"
		render_explicit_query "${tmp_output}" "${manual_query}"
	else
		log::info "Verifying managed changelog"
		render_managed "${tmp_output}" "0"
	fi

	if ! diff -u "${CHANGELOG_FILE}" "${tmp_output}" >/dev/null; then
		log::error "CHANGELOG.md is out of date. Run 'make changelog'."
		diff -u "${CHANGELOG_FILE}" "${tmp_output}" || true
		exit 1
	fi

	log::success "Changelog is up to date"
}

print_usage() {
	echo "Usage: $0 <generate|preview|verify|state-print|state-reset>"
}

case "${ACTION}" in
generate)
	generate_changelog
	;;
preview)
	preview_changelog
	;;
verify)
	verify_changelog
	;;
state-print)
	state_print
	;;
state-reset)
	state_reset
	;;
*)
	print_usage
	exit 1
	;;
esac
