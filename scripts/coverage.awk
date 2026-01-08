#!/usr/bin/env awk
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