#!/bin/bash
# Portforward hack for CircleCI remote docker
set -o errexit
set -o nounset
set -o pipefail
set -o errtrace

if [[ ${1:-} = start ]]; then
  docker run -d -it \
         --name portforward --net=host \
         --entrypoint /bin/sh \
         bobrik/socat -c "while true; do sleep 1000; done"
elif [[ ${1} ]]; then
  socat "TCP-LISTEN:${1},reuseaddr,fork" \
        EXEC:"'docker exec -i portforward socat STDIO TCP-CONNECT:localhost:${1}'"
else
  echo "Must specify either start or the port number" >&2
fi
