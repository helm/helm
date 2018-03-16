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
set -exuo pipefail

covermode=${COVERMODE:-atomic}
profile=$(mktemp /tmp/coverage.XXXX)

hash goveralls 2>/dev/null || go get github.com/mattn/goveralls

go test -coverprofile "${profile}" -covermode "$covermode" ./...
go tool cover -func "${profile}"

case "${1-}" in
  --html)
    go tool cover -html "${profile}"
    ;;
  --coveralls)
    goveralls -coverprofile "${profile}" -service circle-ci
    ;;
esac
