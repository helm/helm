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

case "${CIRCLE_NODE_INDEX-0}" in
  0)
    echo "Running 'make test-unit'"
    make test-unit
    ;;
  1)
    echo "Running 'make test-style'"
    make test-style
    ;;
  2)
    echo "Running 'make docker-build'"
    make docker-build
    ;;
esac
