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

# Tear down kubernetes in docker

set -eo pipefail
[[ "$TRACE" ]] && set -x

HELM_ROOT="${BASH_SOURCE[0]%/*}/.."
source "${HELM_ROOT}/scripts/common.sh"
source "${HELM_ROOT}/scripts/docker.sh"

KUBE_PORT=${KUBE_PORT:-8080}
KUBE_HOST=${KUBE_HOST:-$DOCKER_HOST_IP}
KUBE_HOST=${KUBE_HOST:-localhost}
KUBECTL="kubectl -s ${KUBE_HOST}:${KUBE_PORT}"

delete_kube_resources() {
  echo "Deleting resources in kubernetes..."

  $KUBECTL delete replicationcontrollers,services,pods,secrets --all > /dev/null 2>&1 || :
  $KUBECTL delete replicationcontrollers,services,pods,secrets --all --namespace=kube-system > /dev/null 2>&1 || :
  $KUBECTL delete namespace kube-system > /dev/null 2>&1 || :
}

delete_hyperkube_containers() {
  echo "Stopping main kubelet..."

  docker stop helm_kubelet > /dev/null 2>&1 || true
  docker rm --force --volumes helm_kubelet > /dev/null 2>&1 || true

  echo "Stopping remaining kubernetes containers..."

  local kube_containers=$(docker ps -aqf "name=k8s_")
  if [ ! -z "$kube_containers" ]; then
    docker stop $kube_containers > /dev/null 2>&1
    docker wait $kube_containers > /dev/null 2>&1
    docker rm --force --volumes $kube_containers > /dev/null 2>&1
  fi
}

main() {
  echo "Bringing down the kube..."

  delete_kube_resources
  delete_hyperkube_containers

  echo "done."
}

main "$@"

exit 0
