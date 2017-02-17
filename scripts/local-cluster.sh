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

# Bash 'Strict Mode'
# http://redsymbol.net/articles/unofficial-bash-strict-mode
set -euo pipefail
IFS=$'\n\t'

HELM_ROOT="${BASH_SOURCE[0]%/*}/.."
cd "$HELM_ROOT"

# Globals ----------------------------------------------------------------------

KUBE_VERSION=${KUBE_VERSION:-}
KUBE_PORT=${KUBE_PORT:-8080}
KUBE_CONTEXT=${KUBE_CONTEXT:-docker}
KUBECTL=${KUBECTL:-kubectl}
ENABLE_CLUSTER_DNS=${KUBE_ENABLE_CLUSTER_DNS:-true}
LOG_LEVEL=${LOG_LEVEL:-2}

# Helper Functions -------------------------------------------------------------

# Display error message and exit
error_exit() {
  echo "error: ${1:-"unknown error"}" 1>&2
  exit 1
}

# Checks if a command exists.  Returns 1 or 0
command_exists() {
  hash "${1}" 2>/dev/null
}

# fetch url using wget or curl and print to stdout
fetch_url() {
  local url="$1"
  if command_exists wget; then
    curl -sSL "$url"
  elif command_exists curl; then
    wget -qO- "$url"
  else
    error_exit "Couldn't find curl or wget.  Bailing out."
  fi
}

# Program Functions ------------------------------------------------------------

# Check host platform and docker host
verify_prereqs() {
  echo "Verifying Prerequisites...."

  case "$(uname -s)" in
    Darwin)
      host_os=darwin
      ;;
    Linux)
      host_os=linux
      ;;
    *)
      error_exit "Unsupported host OS.  Must be Linux or Mac OS X."
      ;;
  esac

  case "$(uname -m)" in
    x86_64*)
      host_arch=amd64
      ;;
    i?86_64*)
      host_arch=amd64
      ;;
    amd64*)
      host_arch=amd64
      ;;
    arm*)
      host_arch=arm
      ;;
    i?86*)
      host_arch=x86
      ;;
    s390x*)
      host_arch=s390x
      ;;
    ppc64le*)
      host_arch=ppc64le
      ;;
    *)
      error_exit "Unsupported host arch. Must be x86_64, 386, arm, s390x or ppc64le."
      ;;
  esac


  command_exists docker || error_exit "You need docker"

  if ! docker info >/dev/null 2>&1 ; then
    error_exit "Can't connect to 'docker' daemon."
  fi

  if docker inspect kubelet >/dev/null 2>&1 ; then
    error_exit "Kubernetes is already running"
  fi

  $KUBECTL version --client >/dev/null || download_kubectl
}

# Get the latest stable release tag
get_latest_version_number() {
  local channel="stable"
  if [[ -n "${ALPHA:-}" ]]; then
    channel="latest"
  fi
  local latest_url="https://storage.googleapis.com/kubernetes-release/release/${channel}.txt"
  fetch_url "$latest_url"
}

# Detect ip address od docker host
detect_docker_host_ip() {
  if [[ -n "${DOCKER_HOST:-}" ]]; then
    awk -F'[/:]' '{print $4}' <<< "$DOCKER_HOST"
  else
    ifconfig docker0 \
      | grep -Eo 'inet (addr:)?([0-9]*\.){3}[0-9]*' \
      | grep -Eo '([0-9]*\.){3}[0-9]*' >/dev/null 2>&1 || :
  fi
}

# Set KUBE_MASTER_IP from docker host ip.  Defaults to localhost
set_master_ip() {
  local docker_ip

  if [[ -z "${KUBE_MASTER_IP:-}" ]]; then
    docker_ip=$(detect_docker_host_ip)
    if [[ -n "${docker_ip}" ]]; then
      KUBE_MASTER_IP="${docker_ip}"
    else
      KUBE_MASTER_IP=localhost
    fi
  fi
}

# Start dockerized kubelet
start_kubernetes() {
  echo "Starting kubelet"

  # Enable dns
  if [[ "${ENABLE_CLUSTER_DNS}" = true ]]; then
    dns_args="--cluster-dns=10.0.0.10 --cluster-domain=cluster.local"
  else
    # DNS server for real world hostnames.
    dns_args="--cluster-dns=8.8.8.8"
  fi

  local start_time=$(date +%s)

  docker run \
    --name=kubelet \
    --volume=/:/rootfs:ro \
    --volume=/sys:/sys:ro \
    --volume=/var/lib/docker/:/var/lib/docker:rw \
    --volume=/var/lib/kubelet/:/var/lib/kubelet:rw,rslave \
    --volume=/var/run:/var/run:rw \
    --net=host \
    --pid=host \
    --privileged=true \
    -d \
    gcr.io/google_containers/hyperkube-amd64:${KUBE_VERSION} \
      /hyperkube kubelet \
        --containerized \
        --hostname-override="127.0.0.1" \
        --api-servers=http://localhost:${KUBE_PORT} \
        --config=/etc/kubernetes/manifests \
        --allow-privileged=true \
        ${dns_args} \
        --v=${LOG_LEVEL} >/dev/null

  until $KUBECTL cluster-info &> /dev/null; do
    sleep 1
  done

  if [[ $KUBE_VERSION == "1.2"* ]]; then
    create_kube_system_namespace
    create_kube_dns
  fi

  # We expect to have at least 3 running pods - etcd, master and kube-proxy.
  local attempt=1
  while (($(KUBECTL get pods --all-namespaces --no-headers 2>/dev/null | grep -c "Running") < 3)); do
    echo -n "."
    sleep $(( attempt++ ))
  done
  echo

  echo "Started master components in $(($(date +%s) - start_time)) seconds."
}

# Open kubernetes master api port.
setup_firewall() {
  [[ -n "${DOCKER_MACHINE_NAME:-}" ]] || return

  echo "Adding iptables hackery for docker-machine..."

  local machine_ip
  machine_ip=$(docker-machine ip "$DOCKER_MACHINE_NAME")
  local iptables_rule="PREROUTING -p tcp -d ${machine_ip} --dport ${KUBE_PORT} -j DNAT --to-destination 127.0.0.1:${KUBE_PORT}"

  if ! docker-machine ssh "${DOCKER_MACHINE_NAME}" "sudo /usr/local/sbin/iptables -t nat -C ${iptables_rule}" &> /dev/null; then
    docker-machine ssh "${DOCKER_MACHINE_NAME}" "sudo /usr/local/sbin/iptables -t nat -I ${iptables_rule}"
  fi
}

# Create kube-system namespace in kubernetes
create_kube_system_namespace() {
  $KUBECTL create namespace kube-system >/dev/null
}

# Activate skydns in kubernetes and wait for pods to be ready.
create_kube_dns() {
  [[ "${ENABLE_CLUSTER_DNS}" = true ]] || return

  local start_time=$(date +%s)

  echo "Setting up cluster dns..."

  $KUBECTL create -f ./scripts/cluster/skydns.yaml >/dev/null

  echo "Waiting for cluster DNS to become available..."

  local attempt=1
  until $KUBECTL get pods --no-headers --namespace kube-system --selector=k8s-app=kube-dns 2>/dev/null | grep "Running" &>/dev/null; do
    echo -n "."
    sleep $(( attempt++ ))
  done
  echo
  echo "Started DNS in $(($(date +%s) - start_time)) seconds."
}

# Generate kubeconfig data for the created cluster.
generate_kubeconfig() {
  local cluster_args=(
      "--server=http://${KUBE_MASTER_IP}:${KUBE_PORT}"
      "--insecure-skip-tls-verify=true"
  )

  $KUBECTL config set-cluster "${KUBE_CONTEXT}" "${cluster_args[@]}" >/dev/null
  $KUBECTL config set-context "${KUBE_CONTEXT}" --cluster="${KUBE_CONTEXT}" >/dev/null
  $KUBECTL config use-context "${KUBE_CONTEXT}" >/dev/null

  echo "Wrote config for kubeconfig using context: '${KUBE_CONTEXT}'"
}

# Download kubectl
download_kubectl() {
  echo "Downloading kubectl binary..."

  local output="/usr/local/bin/kubectl"

  kubectl_url="https://storage.googleapis.com/kubernetes-release/release/${KUBE_VERSION}/bin/${host_os}/${host_arch}/kubectl"
  fetch_url "${kubectl_url}" > "${output}"
  chmod a+x "${output}"

  KUBECTL="${output}"
}

# Clean volumes that are left by kubelet
#
# https://github.com/kubernetes/kubernetes/issues/23197
# code stolen from https://github.com/huggsboson/docker-compose-kubernetes/blob/SwitchToSharedMount/kube-up.sh
clean_volumes() {
  if [[ -n "${DOCKER_MACHINE_NAME:-}" ]]; then
    docker-machine ssh "${DOCKER_MACHINE_NAME}" "mount | grep -o 'on /var/lib/kubelet.* type' | cut -c 4- | rev | cut -c 6- | rev | sort -r | xargs --no-run-if-empty sudo umount"
    docker-machine ssh "${DOCKER_MACHINE_NAME}" "sudo rm -Rf /var/lib/kubelet"
    docker-machine ssh "${DOCKER_MACHINE_NAME}" "sudo mkdir -p /var/lib/kubelet"
    docker-machine ssh "${DOCKER_MACHINE_NAME}" "sudo mount --bind /var/lib/kubelet /var/lib/kubelet"
    docker-machine ssh "${DOCKER_MACHINE_NAME}" "sudo mount --make-shared /var/lib/kubelet"
  else
    mount | grep -o 'on /var/lib/kubelet.* type' | cut -c 4- | rev | cut -c 6- | rev | sort -r | xargs --no-run-if-empty sudo umount
    sudo rm -Rf /var/lib/kubelet
    sudo mkdir -p /var/lib/kubelet
    sudo mount --bind /var/lib/kubelet /var/lib/kubelet
    sudo mount --make-shared /var/lib/kubelet
  fi
}

# Helper function to properly remove containers
delete_container() {
  local container=("$@")
  docker stop "${container[@]}" &>/dev/null || :
  docker wait "${container[@]}" &>/dev/null || :
  docker rm --force --volumes "${container[@]}" &>/dev/null || :
}

# Delete master components and resources in kubernetes.
kube_down() {
  echo "Deleting all resources in kubernetes..."
  $KUBECTL delete replicationcontrollers,services,pods,secrets --all >/dev/null 2>&1 || :
  $KUBECTL delete replicationcontrollers,services,pods,secrets --all --namespace=kube-system >/dev/null 2>&1 || :
  $KUBECTL delete namespace kube-system >/dev/null 2>&1 || :

  echo "Stopping kubelet..."
  delete_container kubelet

  echo "Stopping remaining kubernetes containers..."
  local kube_containers=($(docker ps -aqf "name=k8s_"))
  if [[ "${#kube_containers[@]}" -gt 0 ]]; then
    delete_container "${kube_containers[@]}"
  fi
}

# Start a kubernetes cluster in docker.
kube_up() {
  verify_prereqs

  set_master_ip
  clean_volumes
  setup_firewall

  generate_kubeconfig
  start_kubernetes

  $KUBECTL cluster-info
}

KUBE_VERSION=${KUBE_VERSION:-$(get_latest_version_number)}

# Main -------------------------------------------------------------------------

main() {
  case "$1" in
    up|start)
      kube_up
      ;;
    down|stop)
      kube_down
      ;;
    restart)
      kube_down
      kube_up
      ;;
    *)
      echo "Usage: $0 {up|down|restart}"
      ;;
  esac
}

main "${@:-}"

