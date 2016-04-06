#!/bin/bash

set -eo pipefail

[[ "$TRACE" ]] && set -x

HELM_ROOT="${BASH_SOURCE[0]%/*}/.."
source "$HELM_ROOT/scripts/common.sh"

KUBE_PROXY=${KUBE_PROXY:-}
KUBE_PROXY_PORT=${KUBE_PROXY_PORT:-8001}
MANAGER_PORT=${MANAGER_PORT:-8080}

RESOURCIFIER=bin/resourcifier
EXPANDYBIRD=bin/expandybird
MANAGER=bin/manager

require_binary_exists() {
  if ! command -v "$1" >/dev/null 2>&1; then
    error_exit "Cannot find binary for $1. Build binaries by running 'make build'"
  fi
}

kill_service() {
  pkill -f "$1" || true
}

for b in $RESOURCIFIER $EXPANDYBIRD $MANAGER; do
  require_binary_exists $b
  kill_service $b
done

LOGDIR=log
if [[ ! -d $LOGDIR ]]; then
  mkdir $LOGDIR
fi

KUBECTL=$(which kubectl) || error_exit "Cannot find kubectl"

echo "Starting resourcifier..."
nohup $RESOURCIFIER > $LOGDIR/resourcifier.log 2>&1 --kubectl="${KUBECTL}" --port=8082 &

echo "Starting expandybird..."
nohup $EXPANDYBIRD > $LOGDIR/expandybird.log 2>&1 --port=8081 --expansion_binary=expansion/expansion.py &

echo "Starting deployment manager..."
nohup $MANAGER > $LOGDIR/manager.log 2>&1 --port="${MANAGER_PORT}"  --kubectl="${KUBECTL}" --expanderPort=8081 --deployerPort=8082 &

if [[ "$KUBE_PROXY" ]]; then
  echo "Starting kubectl proxy..."
  pkill -f "$KUBECTL proxy"
  nohup "$KUBECTL" proxy --port="${KUBE_PROXY_PORT}" &
  sleep 1s
fi

cat <<EOF
Local manager server is now running on :${MANAGER_PORT}

Logging to ${LOGDIR}

To use helm:

  export HELM_HOST=http://localhost:${MANAGER_PORT}
  For list of commands, run:
  $./bin/helm
  Example Command:
  $./bin/helm repo list

EOF
