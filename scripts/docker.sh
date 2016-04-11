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

docker_detect_host_ip() {
  if [ -n "$DOCKER_HOST" ]; then
    awk -F'[/:]' '{print $4}' <<< "$DOCKER_HOST"
  else
    ifconfig docker0 \
      | grep -Eo 'inet (addr:)?([0-9]*\.){3}[0-9]*' \
      | grep -Eo '([0-9]*\.){3}[0-9]*' >/dev/null 2>&1 || :
  fi
}

DOCKER_HOST_IP=$(docker_detect_host_ip)

is_docker_machine() {
  [[ $(docker-machine active 2>/dev/null) ]]
}

active_docker_machine() {
  if command -v docker-machine >/dev/null 2>&1; then
    docker-machine active
  fi
}

delete_container() {
  local container=("$@")

  docker stop "${container[@]}" &>/dev/null || :
  docker wait "${container[@]}" &>/dev/null || :
  docker rm --force --volumes "${container[@]}" &>/dev/null || :
}

dev_registry() {
  if docker inspect registry >/dev/null 2>&1; then
    docker start registry
  else
    docker run --restart="always" -d -p 5000:5000 --name registry registry:2
  fi
}
