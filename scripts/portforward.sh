#!/bin/bash

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
  exit 1
fi
