VERSION           := 0.1
DOCKER_REGISTRY   := helmpack
SHORT_NAME        := ci
IMAGE             := ${DOCKER_REGISTRY}/${SHORT_NAME}:${VERSION}

.PHONY: image
image:
	docker build -t "${IMAGE}" ./image

.PHONY: push
push:
	docker push "${IMAGE}"

.PHONY: run
run:
	docker run --rm -it \
		-v ${GOPATH}:${GOPATH} \
		-w ${PWD} \
		--entrypoint /bin/bash \
		"${IMAGE}"
