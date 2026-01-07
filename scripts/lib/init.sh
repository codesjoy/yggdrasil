#!/usr/bin/env bash

set -o errexit
set +o nounset
set -o pipefail

# Unset CDPATH so that path interpolation can work correctly
unset CDPATH

# The root of the project
ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)

source "${ROOT_DIR}/scripts/lib/logger.sh"
source "${ROOT_DIR}/scripts/lib/util.sh"
