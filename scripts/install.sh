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


## Ported from Helm Classic
set -eo pipefail -o nounset

function check_platform_arch {
  local supported="linux-amd64\ndarwin-amd64\nlinux-i386"

  if ! echo "${supported}" | grep -q "${PLATFORM}-${ARCH}"; then
    cat <<EOF

No binaries for ${PLATFORM}-${ARCH}. Go to https://github.com/kubernetes/helm
to download the source code.

EOF
  fi
}

VERSION="canary"
PROGRAM="helm"
PLATFORM="$(uname | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"
HELM_BIN_URL_BASE="https://storage.googleapis.com/kubernetes-helm"

if [ "${ARCH}" == "x86_64" ]; then
  ARCH="amd64"
fi

check_platform_arch

HELM_BIN="helm-${VERSION}-${PLATFORM}-${ARCH}.tar.gz"
HELM_SUM="${HELM_BIN}.sha26"
PROGTAR="${PROGRAM}-${VERSION}.tgz"
PRoGSUM="${PROGTAR}.sha256"

echo "Downloading ${HELM_BIN}..."
#curl -o ${PROGRAM}.sha256 -s "${HELM_BIN_URL_BASE}/${HELM_SUM}"
curl -o ${PROGTAR} -s "${HELM_BIN_URL_BASE}/${HELM_BIN}"
#if $(shasum -a 256 ${PROGTAR}) -ne $(cat ${PROGSUM}); then
#  echo "Sums do not match. Aborting for security reasons"
#fi

tar -zxf ${PROGTAR}

# This is sloppy. Should probably handle this in the tar itself.
cp "${PLATFORM}-${ARCH}/${PROGRAM}" .
chmod +x "${PROGRAM}"

cat <<EOF
Helm is ready to sail.

    $ ./${PROGRAM} help

EOF

