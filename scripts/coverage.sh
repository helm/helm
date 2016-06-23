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

COVERDIR=${COVERDIR:-.coverage}
COVERMODE=${COVERMODE:-atomic}
PACKAGES=($(go list $(glide novendor)))

if [[ ! -d "$COVERDIR" ]]; then
  mkdir -p "$COVERDIR"
fi

echo "mode: ${COVERMODE}" > "${COVERDIR}/coverage.out"

for d in "${PACKAGES[@]}"; do
  go test -coverprofile=profile.out -covermode="$COVERMODE" "$d"
  if [ -f profile.out ]; then
    sed "/mode: $COVERMODE/d" profile.out >> "${COVERDIR}/coverage.out"
    rm profile.out
  fi
done

go tool cover -html "${COVERDIR}/coverage.out" -o "${COVERDIR}/coverage.html"
