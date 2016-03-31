#!/usr/bin/env bash
# Copyright 2016 The Kubernetes Authors All rights reserved.
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

set -o errexit
set -o pipefail

readonly ROOTFS="${DIR}/rootfs"

readonly STATIC_TARGETS=(cmd/expandybird cmd/goexpander cmd/manager cmd/resourcifier)
readonly ALL_TARGETS=(${STATIC_TARGETS[@]} cmd/helm)

error_exit() {
  # Display error message and exit
  echo "error: ${1:-"unknown error"}" 1>&2
  exit 1
}

is_osx() {
  [[ "$(uname)" == "Darwin" ]]
}

assign_version() {
  if [[ -z "${VERSION:-}" ]]; then
    VERSION=$(version_from_git)
  fi
}

assign_ldflags() {
  if [[ -z "${LDFLAGS:-}" ]]; then
    LDFLAGS="-s -X ${REPO}/pkg/version.Version=${VERSION}"
  fi
}

version_from_git() {
  local git_tag=$(git describe --tags --abbrev=0 2>/dev/null)
  local git_commit=$(git rev-parse --short HEAD)
  echo "${git_tag}+${git_commit}"
}

build_binary_static() {
  local target="$1"
  local basename="${target##*/}"
  local context="${ROOTFS}/${basename}"

  echo "Building ${target}"
  CGO_ENABLED=0 \
  GOOS=linux \
  GOARCH=amd64 \
  go build \
    -ldflags="${LDFLAGS}" \
    -a -installsuffix cgo \
    -o "${context}/bin/${basename}" \
    "${REPO}/${target}"	
}

build_binary_cross() {
  local target="$1"

  echo "Building ${target}"
  gox -verbose \
    -ldflags="${LDFLAGS}" \
    -os="linux darwin" \
    -arch="amd64 386" \
    -output="bin/{{.OS}}-{{.Arch}}/{{.Dir}}" \
    "${REPO}/${target}"
}

#TODO: accept specific os/arch
build_binaries() {
  local -a targets=($@)
  local build_type="${BUILD_TYPE:-}"

  if [[ ${#targets[@]} -eq 0 ]]; then
    if [[ ${build_type} == STATIC ]]; then
      targets=("${STATIC_TARGETS[@]}")
    else
      targets=("${ALL_TARGETS[@]}")
    fi
  fi

  assign_version
  assign_ldflags

  for t in "${targets[@]}"; do
    if [[ ${build_type} == STATIC ]]; then
      build_binary_static "$t"
    elif [[ ${build_type} == CROSS ]]; then
      build_binary_cross "$t"
    else    
      build_binary "$t"
    fi
  done
}

build_binary() {
  local target="$1"
  local binary="${target##*/}"
  local outfile="bin/${binary}"

  echo "Building ${target}"
  go build -o "$outfile" -ldflags "$LDFLAGS" "${REPO}/${target}"
}
