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

K8S_VERSION=${K8S_VERSION:-1.2.0}
KUBE_PORT=${KUBE_PORT:-8080}
KUBE_MASTER_IP=${KUBE_MASTER_IP:-$DOCKER_HOST_IP}
KUBE_MASTER_IP=${KUBE_MASTER_IP:-localhost}
KUBECTL="kubectl -s ${KUBE_MASTER_IP}:${KUBE_PORT}"
KUBE_CONTEXT=${KUBE_CONTEXT:-docker}

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

  if ! docker-machine ssh "${machine}" "sudo /usr/local/sbin/iptables -t nat -C ${iptables_rule}"; then
    docker-machine ssh "${machine}" "sudo /usr/local/sbin/iptables -t nat -I ${iptables_rule}"
  fi
}

start_kubernetes() {
  echo "Getting the party going..."

  #if docker ps --filter "name=helm_kubelet" >/dev/null; then
    #error_exit "Kubernetes already running"
  #fi

  docker run \
    --name=helm_kubelet \
    --volume=/:/rootfs:ro \
    --volume=/sys:/sys:ro \
    --volume=/var/lib/docker/:/var/lib/docker:rw \
    --volume=/var/lib/kubelet/:/var/lib/kubelet:rw \
    --volume=/var/run:/var/run:rw \
    --net=host \
    --pid=host \
    --privileged=true \
    -d \
    gcr.io/google_containers/hyperkube-amd64:v${K8S_VERSION} \
    /hyperkube kubelet \
        --containerized \
        --hostname-override="127.0.0.1" \
        --address="0.0.0.0" \
        --api-servers="http://localhost:${KUBE_PORT}" \
        --config=/etc/kubernetes/manifests \
        --cluster-dns=10.0.0.10 \
        --cluster-domain=cluster.local \
        --allow-privileged=true --v=2
}

wait_for_kubernetes() {
  echo "Waiting for Kubernetes cluster to become available..."
  until $($KUBECTL cluster-info &> /dev/null); do
    sleep 1
  done
  echo "Kubernetes cluster is up."
}

create_kube_system_namespace() {
  echo "Creating kube-system namespace..."

  $KUBECTL create -f - << EOF
kind: Namespace
apiVersion: v1
metadata:
  name: kube-system
  labels:
    name: kube-system
EOF
}

create_kube_dns() {
  echo "Setting up internal dns..."

  $KUBECTL --namespace=kube-system create -f - <<EOF
apiVersion: v1
kind: Endpoints
metadata:
  name: kube-dns
  namespace: kube-system
subsets:
- addresses:
  - ip: $DOCKER_HOST_IP
  ports:
  - port: 53
    protocol: UDP
    name: dns

---

kind: Service
apiVersion: v1
metadata:
  name: kube-dns
  namespace: kube-system
spec:
  clusterIP: 10.0.0.10
  ports:
  - name: dns
    port: 53
    protocol: UDP
EOF
}

# Generate kubeconfig data for the created cluster.
create_kubeconfig() {
  local cluster_args=(
      "--server=http://${KUBE_MASTER_IP}:${KUBE_PORT}"
      "--insecure-skip-tls-verify=true"
  )

  kubectl config set-cluster "${KUBE_CONTEXT}" "${cluster_args[@]}"
  kubectl config set-context "${KUBE_CONTEXT}" --cluster="${KUBE_CONTEXT}"
  kubectl config use-context "${KUBE_CONTEXT}"

  echo "Wrote config for ${KUBE_CONTEXT}"
}

main() {
  verify_prereqs

  if is_docker_machine; then
    setup_iptables
  fi

  start_kubernetes
  wait_for_kubernetes

  create_kube_system_namespace
  create_kube_dns
  create_kubeconfig

  $KUBECTL cluster-info
}

main "$@"

exit 0
