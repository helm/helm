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
set -euo pipefail

# Skip on pull request builds
if [[ -n "${CIRCLE_PR_NUMBER:-}" ]]; then
  exit
fi

: ${AZURE_STORAGE_CONNECTION_STRING:?"AZURE_STORAGE_CONNECTION_STRING environment variable is not set"}
: ${AZURE_STORAGE_CONTAINER_NAME:?"AZURE_STORAGE_CONTAINER_NAME environment variable is not set"}

VERSION=
if [[ -n "${CIRCLE_TAG:-}" ]]; then
  VERSION="${CIRCLE_TAG}"
elif [[ "${CIRCLE_BRANCH:-}" == "main" ]]; then
  VERSION="canary"
else
  echo "Skipping deploy step; this is neither a releasable branch or a tag"
  exit
fi

echo "Installing Azure CLI"
echo "deb [arch=amd64] https://packages.microsoft.com/repos/azure-cli/ stretch main" | sudo tee /etc/apt/sources.list.d/azure-cli.list
curl -L https://packages.microsoft.com/keys/microsoft.asc | sudo apt-key add
sudo apt install apt-transport-https
sudo apt update
sudo apt install azure-cli


echo "Building helm binaries"
make build-cross
make dist checksum VERSION="${VERSION}"

echo "Pushing binaries to Azure"
if [[ "${VERSION}" == "canary" ]]; then
  az storage blob upload-batch -s _dist/ -d "$AZURE_STORAGE_CONTAINER_NAME" --pattern 'helm-*' --connection-string "$AZURE_STORAGE_CONNECTION_STRING" --overwrite
else
  az storage blob upload-batch -s _dist/ -d "$AZURE_STORAGE_CONTAINER_NAME" --pattern 'helm-*' --connection-string "$AZURE_STORAGE_CONNECTION_STRING"
fi
