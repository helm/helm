#!/usr/bin/env bash

# Test organization:
# 0XX: Basic commands (init, version, completion, etc)
# 1XX: Local commands (create, package)
# 2XX: Install/delete commands (helm install XXX)
# 3XX: Commands that work with releases (list, get, status, test)
# 4XX: Repository commands (search, fetch)
# 5XX: RESERVED
# 6XX: Advanced commands (plugins, serve, dependency)
# 7XX: Signing and verification (helm package --sign)
# 8XX: RESERVED
# 9XX: Teardown

set -e -o pipefail
export NAMESPACE="helm-functional-tests"

# We store Tiller's data in the testing namespace.
export TILLER_NAMESPACE=${NAMESPACE}

# Run a tiller locally, and point all Helm operations to that
# tiller.
#2>/dev/null 1>&2 tiller &
tiller &
sleep 3

for s in tests/*.sh; do
  echo "+++ Running " $s
  echo $s
  echo "--- Finished " $s " with code " $?
done

function cleanup {
  echo "Cleaning up"
  pkill tiller || true
  kubectl delete namespace $NAMESPACE || true
}

trap cleanup EXIT
