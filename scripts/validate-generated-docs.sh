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

echo "==> Running generated docs validations <=="

make docs > /dev/null

status="$(git status --porcelain -- ./docs)"
if [[ -n "${status}" ]]; then
  echo
  echo "Auto generated docs are outdated. Run `make docs`"
  echo
  exit 1
fi
