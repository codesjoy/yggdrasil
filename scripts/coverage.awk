#!/usr/bin/env awk
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

# scripts/coverage.awk

/total:/ {
    # 提取覆盖率百分比
    coverage = $NF
    gsub(/%/, "", coverage)

    printf("test coverage is %s%% (quality gate is %s%%)\n", coverage, target)

    if (coverage + 0 < target + 0) {  # 强制转换为数字
        printf("test coverage does not meet expectations: %d%%, please add test cases!\n", target)
        exit 1
    } else {
        printf("test coverage passed!\n")
    }
    exit 0
}