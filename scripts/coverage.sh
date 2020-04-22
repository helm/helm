#!/usr/bin/env bash

# Copyright The Helm Authors.
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

# coverage.sh - Generate test coverage analysis
# 
# Coverage report is sent to coveralls.io if circleci is building the master
# branch.
#
# Usage:
#   coverage.sh [option]
#
# Options:
#   --html         generate html coverage report

set -euo pipefail

covermode=${COVERMODE:-atomic}
coverdir=$(mktemp -d /tmp/coverage.XXXXXXXXXX)
profile="${coverdir}/cover.out"

pushd /
hash goveralls 2>/dev/null || go get github.com/mattn/goveralls
popd

go test -cpu 4 -coverprofile="${profile}" -covermode="$covermode" -coverpkg=./... ./...

go tool cover -func "${profile}"

if [[ "${CIRCLE_BRANCH-}" == 'master' ]]; then
  goveralls -coverprofile="${profile}" -service=circle-ci
fi

if [[ "${1-}" == '--html' ]]; then
    go tool cover -html "${profile}"
fi
