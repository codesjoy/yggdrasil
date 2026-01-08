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


#Define color variables
#Feature

# shellcheck disable=SC2034

if [[ -z "${COLOR_NORMAL+x}" ]]; then
  # style
  readonly COLOR_NORMAL='\033[0m'   COLOR_BOLD='\033[1m'   COLOR_DIM='\033[2m'
  readonly COLOR_ITALIC='\033[3m'   COLOR_UNDER='\033[4m'   COLOR_BLINK='\033[5m'
  readonly COLOR_REVERSE='\033[7m'  COLOR_CONCEAL='\033[8m'

  # foreground
  readonly COLOR_BLACK='\033[30m'   COLOR_RED='\033[31m'   COLOR_GREEN='\033[32m'
  readonly COLOR_YELLOW='\033[33m'  COLOR_BLUE='\033[34m'  COLOR_MAGENTA='\033[35m'
  readonly COLOR_CYAN='\033[36m'    COLOR_WHITE='\033[37m'

  # background
  readonly COLOR_BG_BLACK='\033[40m'  COLOR_BG_RED='\033[41m'    COLOR_BG_GREEN='\033[42m'
  readonly COLOR_BG_YELLOW='\033[43m' COLOR_BG_BLUE='\033[44m'   COLOR_BG_MAGENTA='\033[45m'
  readonly COLOR_BG_CYAN='\033[46m'   COLOR_BG_WHITE='\033[47m'
fi

# Print colors
color::print_color()
{
  # 2026 建议：使用 printf 代替 echo -e，因为 printf 在不同 Unix 系统间行为更一致
  printf "\n${COLOR_BG_MAGENTA}--back-color:${COLOR_NORMAL}\n"
  printf "bblack; bgreen; bblue; bcyan; bred; byellow; bmagenta; bwhite\n\n"

  printf "${COLOR_RED}--font-color:${COLOR_NORMAL}\n"
  printf "black; red; green; yellow; blue; magenta; cyan; white\n\n"

  printf "${COLOR_BOLD}--font-style:${COLOR_NORMAL}\n"
  printf "normal; italic; reverse; bold; dim; blink; under\n\n"
}