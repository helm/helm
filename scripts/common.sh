#!/usr/bin/env bash
set -o errexit
set -o pipefail

readonly ALL_TARGETS=(cmd/dm cmd/expandybird cmd/manager cmd/resourcifier)

error_exit() {
  # Display error message and exit
  echo "error: ${1:-"unknown error"}" 1>&2
  exit 1
}

version_from_git() {
  local git_tag=$(git describe --tags --abbrev=0 2>/dev/null)
  local git_commit=$(git rev-parse --short HEAD)
  echo "${git_tag}+${git_commit}"
}

build_binary_cross() {
  local target="$1"

  echo "Building ${target}"
  gox -verbose \
    -ldflags="${LDFLAGS}" \
    -os="linux darwin" \
    -arch="amd64 386" \
    -output="bin/{{.OS}}-{{.Arch}}/{{.Dir}}" "${REPO}/${target}"
}

build_binaries() {
  local -a targets=($@)

  if [[ ${#targets[@]} -eq 0 ]]; then
    targets=("${ALL_TARGETS[@]}")
  fi

  for t in "${targets[@]}"; do
    build_binary "$t"
  done
}

build_binary() {
  local target="$1"
  local binary="${target##*/}"
  local outfile="bin/${binary}"

  echo "Building ${target}"
  go build -o "$outfile" -ldflags "$LDFLAGS" "${REPO}/${target}"
}
