#!/bin/sh

set -eu

: "${ALLOY_BUILDER_CAPTURE_ARGS:?must name the argument capture file}"
: "${ALLOY_BUILDER_CAPTURE_CONFIG:?must name the config capture file}"
: "${ALLOY_BUILDER_CAPTURE_BUILD_TAGS_ENV:?must name the environment capture file}"

if build_tags_env=$(printenv 'dist.build_tags'); then
	printf '%s\n' "${build_tags_env}" >"${ALLOY_BUILDER_CAPTURE_BUILD_TAGS_ENV}"
else
	printf '%s\n' '<unset>' >"${ALLOY_BUILDER_CAPTURE_BUILD_TAGS_ENV}"
fi

config_path=
: >"${ALLOY_BUILDER_CAPTURE_ARGS}"

while [ "$#" -gt 0 ]; do
	printf '%s\n' "$1" >>"${ALLOY_BUILDER_CAPTURE_ARGS}"
	case "$1" in
	--config|-c)
		shift
		[ "$#" -gt 0 ] || exit 2
		printf '%s\n' "$1" >>"${ALLOY_BUILDER_CAPTURE_ARGS}"
		config_path=$1
		;;
	--config=*|-c=*)
		config_path=${1#*=}
		;;
	esac
	shift
done

[ -n "${config_path}" ] || exit 2
cp "${config_path}" "${ALLOY_BUILDER_CAPTURE_CONFIG}"
