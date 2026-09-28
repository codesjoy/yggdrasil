#!/bin/sh
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

# Per-module coverage collection and quality gate.
#
# Usage:
#   scripts/coverage.sh collect <module-dir> <coverage-dir> <name>
#   scripts/coverage.sh check   <module-dir> <coverage-dir> <name>
#
# <coverage-dir> must already exist. Artifacts are <coverage-dir>/<name>.out,
# .html and .txt.
#
# Environment:
#   COVERAGE  minimum total coverage percentage (default: 80)
#   GO        go binary (default: go)

set -eu

mode="${1:-}"
module="${2:-}"
coverage_dir="${3:-}"
name="${4:-}"
if [ -z "${mode}" ] || [ -z "${module}" ] || [ -z "${coverage_dir}" ] || [ -z "${name}" ]; then
	echo "usage: coverage.sh <collect|check> <module-dir> <coverage-dir> <name>" >&2
	exit 2
fi

go_bin="${GO:-go}"
threshold="${COVERAGE:-80}"
profile="${coverage_dir}/${name}"

case "${mode}" in
collect)
	cd "${module}"
	# Packages without test files are skipped by go test itself, so there is no
	# need to filter the package list up front.
	GOWORK=off "${go_bin}" test -coverprofile="${profile}.out" -covermode=atomic ./...
	if [ -f "${profile}.out" ]; then
		GOWORK=off "${go_bin}" tool cover -html="${profile}.out" -o "${profile}.html"
	fi
	;;
check)
	if [ ! -f "${profile}.out" ]; then
		echo "==> ${module}: no coverage profile, skipped"
		exit 0
	fi
	cd "${module}"
	GOWORK=off "${go_bin}" tool cover -func="${profile}.out" >"${profile}.txt"
	pct=""
	while IFS= read -r line; do
		# shellcheck disable=SC2086  # intentional word split on the tab-separated fields
		set -- ${line}
		if [ "${1:-}" = "total:" ]; then
			pct="${3:-}"
		fi
	done <"${profile}.txt"
	pct="${pct%\%*}"
	if [ -z "${pct}" ]; then
		echo "==> ${module}: no coverage total, skipped"
		exit 0
	fi
	if [ "${pct%.*}" -lt "${threshold}" ]; then
		echo "${module}: coverage ${pct}% is below the ${threshold}% gate" >&2
		exit 1
	fi
	echo "${module}: coverage ${pct}% (gate ${threshold}%)"
	;;
*)
	echo "unknown mode: ${mode}" >&2
	exit 2
	;;
esac
