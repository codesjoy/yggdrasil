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

set -eu

repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
go_bin=${GO:-go}
tmp_dir=$(mktemp -d "${TMPDIR:-/tmp}/yggdrasil-grpc-compat.XXXXXX")
trap 'rm -rf "$tmp_dir"' EXIT HUP INT TERM

cp "$repo_root/go.mod" "$tmp_dir/compat.mod"
cp "$repo_root/go.sum" "$tmp_dir/compat.sum"

cd "$repo_root"
GOWORK=off GOTOOLCHAIN=go1.26.0 "$go_bin" mod edit \
	-modfile="$tmp_dir/compat.mod" \
	-go=1.26.0 \
	-require=google.golang.org/grpc@v1.85.0

GOWORK=off GOTOOLCHAIN=go1.26.0 "$go_bin" test \
	-modfile="$tmp_dir/compat.mod" \
	-mod=mod \
	-count=1 \
	./transport/protocol/grpc/... \
	./transport/runtime/server/...
