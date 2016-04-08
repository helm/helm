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
KUBE_MASTER_IP=${KUBE_MASTER_IP:-$DOCKER_HOST_IP}
KUBE_MASTER_IP=${KUBE_MASTER_IP:-localhost}
KUBECTL="kubectl -s ${KUBE_MASTER_IP}:${KUBE_PORT}"

delete_kube_resources() {
  echo "Deleting resources in kubernetes..."

  $KUBECTL delete replicationcontrollers,services,pods,secrets --all > /dev/null 2>&1 || :
  $KUBECTL delete replicationcontrollers,services,pods,secrets --all --namespace=kube-system > /dev/null 2>&1 || :
  $KUBECTL delete namespace kube-system > /dev/null 2>&1 || :
}

delete_hyperkube_containers() {
  echo "Stopping kubelet..."
  delete_container kubelet

  echo "Stopping remaining kubernetes containers..."
  local kube_containers=($(docker ps -aqf "name=k8s_"))
  if [[ "${#kube_containers[@]}" -gt 0 ]]; then
    delete_container "${kube_containers[@]}"
  fi

  echo "Stopping etcd..."
  delete_container etcd
}

detect_master() {
  local cc=$(kubectl config view -o jsonpath="{.current-context}")
  local cluster=$(kubectl config view -o jsonpath="{.contexts[?(@.name == \"${cc}\")].context.cluster}")
  kubectl config view -o jsonpath="{.clusters[?(@.name == \"${cluster}\")].cluster.server}"
}

main() {
  if [ "$1" != "--force" ]; then
    echo "WARNING: You are about to destroy kubernetes on $(detect_master)"
    read -p "Press [Enter] key to continue..."
  fi

  echo "Bringing down the kube..."

  delete_kube_resources
  delete_hyperkube_containers

  echo "done."
}

main "$@"

exit 0
