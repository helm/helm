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

set -o errexit
set -o nounset
set -o pipefail
set -o xtrace

IMAGE_REPO=${IMAGE_REPO:-gcr.io/kubernetes-helm/tiller}
IMAGE_TAG=${IMAGE_TAG:-canary}
TMP_IMAGE_PATH=${TMP_IMAGE_PATH:-/tmp/image.tar}
NODE_PATTERN=${NODE_PATTERN:-"kube-node-"}


function import-image {
    docker save ${IMAGE_REPO}:${IMAGE_TAG} -o "${TMP_IMAGE_PATH}"

	for node in `docker ps --format "{{.Names}}" | grep ${NODE_PATTERN}`;
	do
		docker cp "${TMP_IMAGE_PATH}" $node:/image.tar
		docker exec -ti "$node" docker load -i /image.tar
	done

    set +o xtrace
    echo "Finished copying docker image to dind nodes"
}

import-image
