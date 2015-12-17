#!/usr/bin/env bash
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

set -euo pipefail

PLATFORM=$(uname | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

STORAGE_URL=http://get-dm.storage.googleapis.com
ZIP=dm-latest-${PLATFORM}-${ARCH}.zip

echo "Downloading ${ZIP}..."
curl -Ls "${STORAGE_URL}/${ZIP}" -O

unzip -qo ${ZIP}
rm ${ZIP}

chmod +x dm

cat <<EOF

dm is now available in your current directory.

Before using it, please install the Deployment Manager service in your
kubernetes cluster by running

  $ kubectl create -f install.yaml

To get started, run:

  $ ./dm

EOF

