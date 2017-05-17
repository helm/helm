#!/usr/bin/env bash

# Copyright 2017 The Kubernetes Authors All rights reserved.
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

source scripts/util.sh

if LANG=C sed --help 2>&1 | grep -q GNU; then
  SED="sed"
elif which gsed &>/dev/null; then
  SED="gsed"
else
  echo "Failed to find GNU sed as sed or gsed. If you are on Mac: brew install gnu-sed." >&2
  exit 1
fi

kube::util::ensure-temp-dir

export HELM_NO_PLUGINS=1

# Reset Helm Home because it is used in the generation of docs.
OLD_HELM_HOME=${HELM_HOME:-}
HELM_HOME="$HOME/.helm"
bin/helm init --client-only
mkdir -p ${KUBE_TEMP}/docs/helm
bin/helm docs --dir ${KUBE_TEMP}/docs/helm
HELM_HOME=$OLD_HELM_HOME

FILES=$(find ${KUBE_TEMP} -type f)

${SED} -i -e "s:${HOME}:~:" ${FILES}

for i in ${FILES}; do
  ret=0
  truepath=$(echo ${i} | ${SED} "s:${KUBE_TEMP}/::")
  diff -NauprB -I 'Auto generated' "${i}" "${truepath}" > /dev/null || ret=$?
  if [[ $ret -ne 0 ]]; then
    echo "${truepath} changed. Updating.."
    cp "${i}" "${truepath}"
  fi
done
