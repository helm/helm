MUTABLE_VERSION ?= canary

GIT_COMMIT := $(shell git rev-parse HEAD)
GIT_SHA := $(shell git rev-parse --short HEAD)
GIT_TAG := $(shell git describe --tags --abbrev=0 2>/dev/null)
GIT_DIRTY = $(shell test -n "`git status --porcelain`" && echo "dirty" || echo "clean")

ifdef VERSION
	DOCKER_VERSION = $(VERSION)
	BINARY_VERSION = $(VERSION)
endif

DOCKER_VERSION ?= git-${GIT_SHA}
BINARY_VERSION ?= ${GIT_TAG}-${GIT_SHA}

IMAGE := ${DOCKER_REGISTRY}/${IMAGE_PREFIX}/${SHORT_NAME}:${DOCKER_VERSION}
MUTABLE_IMAGE := ${DOCKER_REGISTRY}/${IMAGE_PREFIX}/${SHORT_NAME}:${MUTABLE_VERSION}

LDFLAGS += -X k8s.io/helm/pkg/version.Version=${GIT_TAG}
LDFLAGS += -X k8s.io/helm/pkg/version.GitCommit=${GIT_COMMIT}
LDFLAGS += -X k8s.io/helm/pkg/version.GitTreeState=${GIT_DIRTY}

DOCKER_PUSH = docker push
ifeq ($(DOCKER_REGISTRY),gcr.io)
	DOCKER_PUSH = gcloud docker push
endif

info:
	@echo "Build tag:       ${DOCKER_VERSION}"
	@echo "Registry:        ${DOCKER_REGISTRY}"
	@echo "Immutable tag:   ${IMAGE}"
	@echo "Mutable tag:     ${MUTABLE_IMAGE}"

.PHONY: docker-push
docker-push: docker-mutable-push docker-immutable-push

.PHONY: docker-immutable-push
docker-immutable-push:
	${DOCKER_PUSH} ${IMAGE}

.PHONY: docker-mutable-push
docker-mutable-push:
	${DOCKER_PUSH} ${MUTABLE_IMAGE}
