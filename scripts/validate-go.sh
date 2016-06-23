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
set -euo pipefail

readonly  reset=$(tput sgr0)
readonly    red=$(tput bold; tput setaf 1)
readonly  green=$(tput bold; tput setaf 2)
readonly yellow=$(tput bold; tput setaf 3)

exit_code=0

find_go_files() {
  find . -type f -name "*.go" | grep -v vendor
}

hash golint 2>/dev/null || go get -u github.com/golang/lint/golint
hash godir 2>/dev/null || go get -u github.com/Masterminds/godir

echo "==> Running golint..."
for pkg in $(godir pkgs | grep -v proto); do
  golint_out=$(golint "$pkg" 2>&1)
  if [[ -n "$golint_out" ]]; then
    echo "${yellow}${golint_out}${reset}"
  fi
done

echo "==> Running go vet..."
echo -n "$red"
go vet $(godir pkgs) 2>&1 | grep -v "^exit status " || exit_code=${PIPESTATUS[0]}
echo -n "$reset"

echo "==> Running gofmt..."
failed_fmt=$(find_go_files | xargs gofmt -s -l)
if [[ -n "${failed_fmt}" ]]; then
  echo -n "${red}"
  echo "gofmt check failed:"
  echo "$failed_fmt"
  gofmt -s -d "${failed_fmt}"
  echo -n "${reset}"
  exit_code=1
fi

exit ${exit_code}
