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

# Start a kubenetes cluster in docker
#
# Tested on darwin using docker-machine and linux

set -eo pipefail
[[ "$TRACE" ]] && set -x

HELM_ROOT="${BASH_SOURCE[0]%/*}/.."
source "${HELM_ROOT}/scripts/common.sh"
source "${HELM_ROOT}/scripts/docker.sh"

K8S_VERSION=${K8S_VERSION:-1.2.1}
KUBE_PORT=${KUBE_PORT:-8080}
KUBE_MASTER_IP=${KUBE_MASTER_IP:-$DOCKER_HOST_IP}
KUBE_MASTER_IP=${KUBE_MASTER_IP:-localhost}
KUBE_CONTEXT=${KUBE_CONTEXT:-docker}

KUBECTL="kubectl -s ${KUBE_MASTER_IP}:${KUBE_PORT}"

require_command() {
  if ! command -v "$1" >/dev/null 2>&1; then
    error_exit "Cannot find command ${1}"
  fi
}

verify_prereqs() {
  echo "Verifying Prerequisites...."

  require_command docker
  require_command kubectl

  if is_osx; then
    require_command docker-machine
  fi

  if ! docker info > /dev/null 2>&1 ; then
    error_exit "Can't connect to 'docker' daemon.  please fix and retry."
  fi

  if [[ ! $(docker version --format {{.Server.Version}})  == "1.10.3" ]]; then
    error_exit "docker version 1.10.3 is required"
  fi

  echo "You are golden, carry on..."
}

setup_iptables() {
  local machine=$(active_docker_machine)
  if [ -z "$machine" ]; then
    return
  fi

  echo "Adding iptables hackery for docker-machine..."

  local machine_ip=$(docker-machine ip "$machine")
  local iptables_rule="PREROUTING -p tcp -d ${machine_ip} --dport ${KUBE_PORT} -j DNAT --to-destination 127.0.0.1:${KUBE_PORT}"

  if ! docker-machine ssh "${machine}" "sudo /usr/local/sbin/iptables -t nat -C ${iptables_rule}" &> /dev/null; then
    docker-machine ssh "${machine}" "sudo /usr/local/sbin/iptables -t nat -I ${iptables_rule}"
  fi
}

start_kubernetes() {
  echo "Getting the party going..."

  echo "Starting etcd"
  docker run \
    --name=etcd \
    --net=host \
    -d \
    gcr.io/google_containers/etcd:2.2.1 \
    /usr/local/bin/etcd \
      --listen-client-urls=http://127.0.0.1:4001 \
      --advertise-client-urls=http://127.0.0.1:4001 >/dev/null

  echo "Starting kubelet"
  docker run \
    --name=kubelet \
    --volume=/:/rootfs:ro \
    --volume=/sys:/sys:ro \
    --volume=/var/lib/docker/:/var/lib/docker:rw \
    --volume=/var/run:/var/run:rw \
    --volume=/var/lib/kubelet:/var/lib/kubelet:shared \
    --net=host \
    --pid=host \
    --privileged=true \
    -d \
    gcr.io/google_containers/hyperkube-amd64:v${K8S_VERSION} \
    /hyperkube kubelet \
      --hostname-override="127.0.0.1" \
      --address="0.0.0.0" \
      --api-servers=http://localhost:${KUBE_PORT} \
      --config=/etc/kubernetes/manifests-multi \
      --cluster-dns=10.0.0.10 \
      --cluster-domain=cluster.local \
      --allow-privileged=true --v=2 >/dev/null
}

wait_for_kubernetes_cluster() {
  echo "Waiting for Kubernetes cluster to become available..."
  while true; do
    local running_count=$($KUBECTL get pods --all-namespaces --no-headers 2>/dev/null | grep "Running" | wc -l)
    # We expect to have 3 running pods - master, kube-proxy, and dns
    if [ "$running_count" -ge 3 ]; then
      break
    fi
    sleep 1
  done
}

wait_for_kubernetes_master() {
  echo "Waiting for Kubernetes master to become available..."
  until $($KUBECTL cluster-info &> /dev/null); do
    sleep 1
  done
}

create_kube_system_namespace() {
  echo "Creating kube-system namespace..."

  $KUBECTL create -f "${HELM_ROOT}/scripts/cluster/kube-system.yaml" >/dev/null
}

create_kube_dns() {
  echo "Setting up internal dns..."

  $KUBECTL create -f "${HELM_ROOT}/scripts/cluster/skydns.yaml" >/dev/null
}

# Generate kubeconfig data for the created cluster.
create_kubeconfig() {
  local cluster_args=(
      "--server=http://${KUBE_MASTER_IP}:${KUBE_PORT}"
      "--insecure-skip-tls-verify=true"
  )

  kubectl config set-cluster "${KUBE_CONTEXT}" "${cluster_args[@]}" >/dev/null
  kubectl config set-context "${KUBE_CONTEXT}" --cluster="${KUBE_CONTEXT}" >/dev/null
  kubectl config use-context "${KUBE_CONTEXT}" > /dev/null

  echo "Wrote config for kubeconfig using context: '${KUBE_CONTEXT}'"
}

# https://github.com/kubernetes/kubernetes/issues/23197
# code stolen from https://github.com/huggsboson/docker-compose-kubernetes/blob/SwitchToSharedMount/kube-up.sh
cleanup_volumes() {
  local machine=$(active_docker_machine)
  if [ -n "$machine" ]; then
    docker-machine ssh $machine "mount | grep -o 'on /var/lib/kubelet.* type' | cut -c 4- | rev | cut -c 6- | rev | sort -r | xargs --no-run-if-empty sudo umount"
    docker-machine ssh $machine "sudo rm -Rf /var/lib/kubelet"
    docker-machine ssh $machine "sudo mkdir -p /var/lib/kubelet"
    docker-machine ssh $machine "sudo mount --bind /var/lib/kubelet /var/lib/kubelet"
    docker-machine ssh $machine "sudo mount --make-shared /var/lib/kubelet"
  else
    mount | grep -o 'on /var/lib/kubelet.* type' | cut -c 4- | rev | cut -c 6- | rev | sort -r | xargs --no-run-if-empty sudo umount
    sudo rm -Rf /var/lib/kubelet
    sudo mkdir -p /var/lib/kubelet
    sudo mount --bind /var/lib/kubelet /var/lib/kubelet
    sudo mount --make-shared /var/lib/kubelet
  fi
}

verify_prereqs
cleanup_volumes

if is_docker_machine; then
  setup_iptables
fi

start_kubernetes
wait_for_kubernetes_master

create_kube_system_namespace
create_kube_dns
wait_for_kubernetes_cluster

create_kubeconfig


$KUBECTL cluster-info
