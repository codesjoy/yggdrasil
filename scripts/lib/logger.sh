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


source "$(dirname "${BASH_SOURCE[0]}")/color.sh"

# 0=DEBUG, 1=INFO, 2=WARN, 3=ERROR
LOG_LEVEL=${LOG_LEVEL:-0}
# automatically detect: disable colors if not terminal output (e.g., redirected to file)
if [[ -t 1 ]]; then LOG_USE_COLOR=true; else LOG_USE_COLOR=false; fi

# --- Internal Interfaces ---
_log_render() {
    local level_str=$1
    local color=$2
    shift 2
    local msg="$*"
    local ts
    ts=$(date '+%Y-%m-%d %H:%M:%S')
    if [[ "$LOG_USE_COLOR" == "true" ]]; then
        printf "${color}%s  %-5s  %s${COLOR_NORMAL}\n" "$ts" "$level_str" "$msg"
    else
        printf "%s  %-5s  %s\n" "$ts" "$level_str" "$msg"
    fi
}

# --- External Interfaces ---
log::debug() { [[ $LOG_LEVEL -le 0 ]] && _log_render "DEBUG" "$COLOR_CYAN" "$@"; }
log::info()  { [[ $LOG_LEVEL -le 1 ]] && _log_render "INFO"  "$COLOR_GREEN" "$@"; }
log::warn()  { [[ $LOG_LEVEL -le 2 ]] && _log_render "WARN"  "$COLOR_YELLOW" "$@"; }
log::error() { [[ $LOG_LEVEL -le 3 ]] && _log_render "ERROR" "$COLOR_RED" "$@"; }
