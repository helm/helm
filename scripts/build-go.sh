#!/usr/bin/env bash
set -o errexit
set -o pipefail

[[ "$TRACE" ]] && set -x

readonly REPO=github.com/kubernetes/deployment-manager
readonly DIR="${GOPATH}/src/${REPO}"

source "${DIR}/scripts/common.sh"

build_binaries "$@"

exit 0
