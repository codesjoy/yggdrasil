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

# Prints one workspace module directory per line (absolute paths).
#
# Usage:
#   scripts/modules.sh                selected modules (examples excluded by default)
#   scripts/modules.sh --all          every module in the workspace
#   scripts/modules.sh --no-examples  selected modules, examples always excluded
#
# Environment:
#   MODULES          explicit module directories; bypasses all filtering
#   INCLUDE_EXAMPLES 1 to keep the example modules (ignored by --no-examples)
#
# Module roots are the directories that contain a go.mod file, discovered
# through git rather than find/sed. The workspace file (go.work) is a local
# convenience and is deliberately not required.

set -eu

mode=selected
case "${1:-}" in
--all) mode=all ;;
--no-examples) mode=no-examples ;;
"") ;;
*)
	echo "usage: ${0##*/} [--all|--no-examples]" >&2
	exit 2
	;;
esac

# An explicit MODULES list wins, but only when no mode flag was given: Task
# exports its own evaluated variables into the environment of `sh:` commands,
# so MODULES is also visible to the --all / --no-examples invocations and must
# not override them.
if [ -z "${1:-}" ] && [ -n "${MODULES:-}" ]; then
	printf '%s\n' ${MODULES}
	exit 0
fi

exclude_examples=0
case "${mode}" in
all) ;;
no-examples) exclude_examples=1 ;;
*) [ "${INCLUDE_EXAMPLES:-0}" = "1" ] || exclude_examples=1 ;;
esac

repo_root=$(git rev-parse --show-toplevel)
git -C "${repo_root}" ls-files -c -o --exclude-standard -- ':(glob)**/go.mod' | while IFS= read -r go_mod; do
	module_dir=$(dirname "${go_mod}")
	if [ "${module_dir}" = "." ]; then
		module_dir="${repo_root}"
	else
		module_dir="${repo_root}/${module_dir}"
	fi
	if [ "${exclude_examples}" = "1" ]; then
		case "${module_dir}" in
		*/examples | */examples/*) continue ;;
		esac
	fi
	printf '%s\n' "${module_dir}"
done
