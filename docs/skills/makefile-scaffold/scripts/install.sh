#!/usr/bin/env bash
# Install this skill into a target skills directory.

set -o errexit
set -o nounset
set -o pipefail

skill_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
src="${skill_dir}"

dest=""

usage() {
  cat <<'EOF'
Usage:
  bash scripts/install.sh --dest /path/to/skills/makefile-scaffold
  # or: bash /path/to/makefile-scaffold/scripts/install.sh --dest /path/to/skills/makefile-scaffold

This script copies the skill folder for local use. It is safe to re-run.
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --dest)
      dest="$2"
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "unknown arg: $1" >&2
      usage >&2
      exit 2
      ;;
  esac
done

if [[ -z "${dest}" ]]; then
  echo "error: --dest is required" >&2
  usage >&2
  exit 2
fi

mkdir -p "$(dirname "${dest}")"

if command -v rsync >/dev/null 2>&1; then
  rsync -a --delete --exclude '.DS_Store' "${src}/" "${dest}/"
else
  rm -rf "${dest}"
  cp -R "${src}" "${dest}"
fi

echo "installed makefile-scaffold to: ${dest}"
