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

# Bash 'Strict Mode'
# http://redsymbol.net/articles/unofficial-bash-strict-mode
set -euo pipefail
IFS=$'\n\t'

HELM_ROOT="${BASH_SOURCE[0]%/*}/.."
cd "$HELM_ROOT"

run_unit_test() {
  if [[ "${CIRCLE_BRANCH-}" == "master" ]]; then
    echo "Running unit tests with coverage'"
    ./scripts/coverage.sh --coveralls
  else
    echo "Running 'unit tests'"
    make test-unit
  fi
}

run_style_check() {
  echo "Running 'make test-style'"
  make test-style
}

run_docs_check() {
  echo "Running 'make verify-docs'"
  make verify-docs
}

# Build to ensure packages are compiled
echo "Running 'make build'"
make build

case "${CIRCLE_NODE_INDEX-0}" in
  0) run_unit_test   ;;
  1) run_style_check ;;
  2) run_docs_check  ;;
esac
