#!/usr/bin/env bash
#
# Copyright 2015 The Kubernetes Authors All rights reserved.
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

# Run this from deployment-manager root to build and push the dm client plus
# kubernetes install config into the publicly readable GCS bucket gs://get-dm.
#
# Must have EDIT permissions on the dm-k8s-prod GCP project.

set -euo pipefail

DEFAULT_TAG=v1.2
DEFAULT_BINARY=${GOPATH}/bin/dm
DEFAULT_PLATFORM=$(uname | tr '[:upper:]' '[:lower:]')
DEFAULT_ARCH=$(uname -m)

STORAGE_BUCKET=gs://get-dm
ZIP=dm-${TAG:-DEFAULT_TAG}-${PLATFORM:-DEFAULT_PLATFORM}-${ARCH:-DEFAULT_ARCH}.zip

echo "Building..."
make

echo "Zipping ${ZIP}..."
zip -j ${ZIP} ${BINARY:-DEFAULT_BINARY} install.yaml

echo "Uploading ${ZIP} to ${STORAGE_BUCKET}..."
gsutil cp ${ZIP} ${STORAGE_BUCKET}
rm ${ZIP}

echo "Done."

