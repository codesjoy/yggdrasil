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

if [[ $# -ne 1 ]]; then
	echo "Error: commit message file path is required" >&2
	exit 1
fi

commit_msg_file="$1"
raw_commit_msg="$(cat "${commit_msg_file}")"
title_line="$(printf '%s\n' "${raw_commit_msg}" | head -n 1 | sed -e 's/#.*//' -e 's/^[[:space:]]*//' -e 's/[[:space:]]*$//')"

if [[ "${title_line}" =~ ^(Merge|Revert) ]] || [[ -z "${title_line}" ]]; then
	exit 0
fi

if [[ "${title_line}" =~ !: ]]; then
	body="$(printf '%s\n' "${raw_commit_msg}" | sed -e '1d' -e '/^[[:space:]]*$/d' -e '/^[[:space:]]*#/d')"
	if ! grep -qE '^BREAKING[[:space:]]+CHANGE[[:space:]]*:' <<<"${body}"; then
		echo "Error: breaking commit '!' requires a 'BREAKING CHANGE: ...' line in body" >&2
		exit 1
	fi
fi
