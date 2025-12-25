#!/usr/bin/env bash

# This is a tools shell script
# used by Makefile commands

set -o errexit
set -o nounset
set -o pipefail

ROOT_DIR=$(
	cd "$(dirname "${BASH_SOURCE[0]}")" &&
		cd .. &&
		pwd
)

source "${ROOT_DIR}/scripts/util.sh"

LINTER=golangci-lint
LINTER_CONFIG=${ROOT_DIR}/.golangci.yaml
FAILURE_FILE=${ROOT_DIR}/scripts/.hack/.lintcheck_failures
IGNORED_FILE=${ROOT_DIR}/scripts/.hack/.test_ignored_files

all_modules=$(util::find_modules)
failing_modules=()
while IFS='' read -r line; do failing_modules+=("$line"); done < <(cat "$FAILURE_FILE")
ignored_modules=()
while IFS='' read -r line; do ignored_modules+=("$line"); done < <(cat "$IGNORED_FILE")

function lint() {
	if [[ -z "$1" ]]; then
		for mod in $all_modules; do
			lint_one "$mod"
		done
	else
		lint_one "$1"
	fi
}

function lint_one() {
	local mod=$1
	local in_failing
	util::array_contains "$mod" "${failing_modules[*]}" && in_failing=$? || in_failing=$?
	if [[ "$in_failing" -ne "0" ]]; then
		pushd "$mod" >/dev/null &&
			echo "golangci lint $(sed -n 1p go.mod | cut -d ' ' -f2)" &&
			eval "${LINTER} run --timeout=5m --config=${LINTER_CONFIG}"
		popd >/dev/null || exit
	fi
}

function fix() {
	if [[ -z "$1" ]]; then
		for mod in $all_modules; do
			fix_one "$mod"
		done
	else
		fix_one "$1"
	fi
}

function fix_one() {
	local mod=$1
	local in_failing
	util::array_contains "$mod" "${failing_modules[*]}" && in_failing=$? || in_failing=$?
	if [[ "$in_failing" -ne "0" ]]; then
		pushd "$mod" >/dev/null &&
			echo "golangci fix $(sed -n 1p go.mod | cut -d ' ' -f2)" &&
			eval "${LINTER} run -v --fix --timeout=5m --config=${LINTER_CONFIG}"
		popd >/dev/null || exit
	fi
}

function test() {
	if [[ -z "$1" ]]; then
		for mod in $all_modules; do
			test_one "$mod"
		done
	else
		test_one "$1"
	fi
}

function test_one() {
	local mod=$1
	local in_failing
	util::array_contains "$mod" "${ignored_modules[*]}" && in_failing=$? || in_failing=$?
	if [[ "$in_failing" -ne "0" ]]; then
		pushd "$mod" >/dev/null &&
			echo "go test $(sed -n 1p go.mod | cut -d ' ' -f2)" &&
			go test -race ./...
		popd >/dev/null || exit
	fi
}

function test_coverage() {
	echo "" >coverage.out
	if [[ -z "$1" ]]; then
		for mod in $all_modules; do
			test_coverage_one "$mod"
		done
	else
		test_coverage_one "$1"
	fi
}

function test_coverage_one() {
	local mod=$1
	local base
	base=$(pwd)
	local in_failing
	util::array_contains "$mod" "${ignored_modules[*]}" && in_failing=$? || in_failing=$?
	if [[ "$in_failing" -ne "0" ]]; then
		pushd "$mod" >/dev/null &&
			echo "go test $(sed -n 1p go.mod | cut -d ' ' -f2)" &&
			go test -race -coverprofile=profile.out -covermode=atomic ./...
		if [ -f profile.out ]; then
			cat profile.out >>"${base}/coverage.out"
			rm profile.out
		fi
	fi
	popd >/dev/null || exit
}

function coverage_percentage() {
	local mod=$1
	local coverage_file="$mod"coverage.out
	echo "" >"coverage_file"
	go test -race -coverprofile="$coverage_file" "$mod"test/... "$mod"internal/service/...
	percentage=$(go tool cover -func="$coverage_file" | grep total | awk '{print $3}' | sed 's/%//g')
	if ((${percentage%.*} < 90)); then
		echo "coverage percentage is $percentage"
		exit 1
	else
		echo "coverage percentage is $percentage"
		exit 0
	fi
}

function tidy() {
	if [[ -z "$1" ]]; then
		for mod in $all_modules; do
			tidy_one "$mod"
		done
	else
		tidy_one "$1"
	fi
}

function tidy_one() {
	local mod=$1
	pushd "$mod" >/dev/null &&
		echo "go mod tidy $(sed -n 1p go.mod | cut -d ' ' -f2)" &&
		go mod tidy
	popd >/dev/null || exit
}

function help() {
	echo "use: lint, test, test_coverage, fix, tidy"
}

case $1 in
lint)
	shift
	lint "$*"
	;;
test)
	shift
	test "$*"
	;;
test_coverage)
	shift
	test_coverage "$*"
	;;
tidy)
	shift
	tidy "$*"
	;;
fix)
	shift
	fix "$*"
	;;
coverage_percentage)
	shift
	coverage_percentage "$*"
	;;
*)
	help
	;;
esac
