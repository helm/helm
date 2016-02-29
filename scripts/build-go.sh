#!/usr/bin/env bash
set -o errexit
set -o pipefail

[[ "$TRACE" ]] && set -x

readonly REPO=github.com/kubernetes/deployment-manager
readonly DIR="${GOPATH}/src/${REPO}"

source "${DIR}/scripts/common.sh"

if [[ -z "${VERSION:-}" ]]; then
  VERSION=$(version_from_git)
fi

LDFLAGS="-s -X ${REPO}/pkg/version.DeploymentManagerVersion=${VERSION}"

echo "Build version: ${VERSION}"

build_binaries "$@"

exit 0
