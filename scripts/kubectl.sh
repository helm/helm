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

set -eo pipefail
[[ "$TRACE" ]] && set -x

HELM_ROOT="${BASH_SOURCE[0]%/*}/.."
source "${HELM_ROOT}/scripts/common.sh"
source "${HELM_ROOT}/scripts/docker.sh"

KUBE_PORT=${KUBE_PORT:-8080}
KUBE_HOST=${KUBE_HOST:-localhost}

if is_docker_machine; then
  KUBE_HOST=$(docker-machine ip "$(active_docker_machine)")
fi

kubectl -s ${KUBE_HOST}:${KUBE_PORT} "$@"
