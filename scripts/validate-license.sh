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
IFS=$'\n\t'

license='Licensed under the Apache License, Version 2.0'

find_files() {
  find . -type f -not \( \
    \( \
      -wholename './vendor' \
      -o -wholename './pkg/proto' \
      -o -wholename '*testdata*' \
    \) -prune \
  \) \
  \( -name '*.go' -o -name '*.sh' -o -name 'Dockerfile' \) \
  -not \( -exec git check-ignore -q {}+ \;  \)
}


if type "ag" > /dev/null 2>&1 ; then
  # ag automatically skips anything in .gitignore
  ag -L "$license" --go --shell --ignore pkg/proto --ignore testdata --ignore completions.bash
else
  failed=($(find_files | xargs grep -L "$license"))
  if (( ${#failed[@]} > 0 )); then
    echo "Some source files are missing license headers."
    for f in "${failed[@]}"; do
      echo "  $f"
    done
    exit 1
  fi
fi

