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


# Shellcheck wrapper script
# This script runs shellcheck on shell scripts with proper error handling

set -euo pipefail

# Check if shellcheck is installed
if ! command -v shellcheck &> /dev/null; then
    echo "Error: shellcheck is not installed" >&2
    echo "Please install shellcheck to use this script" >&2
    exit 1
fi

# Default shellcheck options
SHELLCHECK_OPTS=(
    --severity=warning
    --exclude=SC2034  # Unused variables
    --exclude=SC2086  # Quote your variables
    --exclude=SC1090  # Can't follow non-constant source
    --exclude=SC1091  # Can't follow non-constant source
)

# If no arguments provided, check all shell scripts in the repo
if [[ $# -eq 0 ]]; then
    find . \( -name "*.sh" -o \( -path "./scripts/*" -type f ! -name "*.go" \) \) -not -path "./.git/*" -not -path "./_output/*" -print0 | xargs -0 shellcheck "${SHELLCHECK_OPTS[@]}"
else
    # Check files provided as arguments
    shellcheck "${SHELLCHECK_OPTS[@]}" "$@"
fi