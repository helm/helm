MUTABLE_VERSION ?= canary
VERSION ?= git-$(shell git rev-parse --short HEAD)

IMAGE := ${DOCKER_REGISTRY}/${IMAGE_PREFIX}/${SHORT_NAME}:${VERSION}
MUTABLE_IMAGE := ${DOCKER_REGISTRY}/${IMAGE_PREFIX}/${SHORT_NAME}:${MUTABLE_VERSION}

info:
	@echo "Build tag:       ${VERSION}"
	@echo "Registry:        ${DOCKER_REGISTRY}"
	@echo "Immutable tag:   ${IMAGE}"
	@echo "Mutable tag:     ${MUTABLE_IMAGE}"

.PHONY: docker-push
docker-push: docker-mutable-push docker-immutable-push

.PHONY: docker-immutable-push
docker-immutable-push:
ifeq ($(DOCKER_REGISTRY),gcr.io)
	gcloud docker push ${IMAGE}
else
	docker push ${IMAGE}
endif

.PHONY: docker-mutable-push
docker-mutable-push:
ifeq ($(DOCKER_REGISTRY),gcr.io)
	gcloud docker push ${MUTABLE_IMAGE}
else
	docker push ${MUTABLE_IMAGE}
endif
