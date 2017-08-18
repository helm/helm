#!/bin/bash

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
