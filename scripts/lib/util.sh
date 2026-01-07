#!/usr/bin/env bash

# This is a common util functions shell script

# arguments: target, item1, item2, item3, ...
# returns 0 if target is in the given items, 1 otherwise.
function util::array_contains() {
	local target="$1"
	shift
	local items="$*"
	# shellcheck disable=SC2048
	for item in ${items[*]}; do
		if [[ "${item}" == "${target}" ]]; then
			return 0
		fi
	done
	return 1
}


function util::parse_params() {
	local params="$1"
	IFS='-' read -ra parts <<<"$params"
	echo "${parts[@]}"
}