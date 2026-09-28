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

# Prints one Go or shell source file per line: every .go/.sh file that git
# tracks or would track and that currently exists in the working tree.
#
# Paths still present in the index but deleted from the working tree are
# skipped, so addlicense never receives a path that is gone. Run from the
# repository root.

set -eu

git ls-files --cached --others --exclude-standard -- '*.go' '*.sh' | while IFS= read -r file; do
	[ -e "${file}" ] || continue
	printf '%s\n' "${file}"
done
