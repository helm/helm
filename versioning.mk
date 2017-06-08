MUTABLE_VERSION := canary

GIT_COMMIT = $(shell git rev-parse HEAD)
GIT_SHA    = $(shell git rev-parse --short HEAD)
GIT_TAG    = $(shell git describe --tags --abbrev=0 --exact-match 2>/dev/null)
GIT_DIRTY  = $(shell test -n "`git status --porcelain`" && echo "dirty" || echo "clean")

ifdef VERSION
	DOCKER_VERSION = $(VERSION)
	BINARY_VERSION = $(VERSION)
endif

DOCKER_VERSION ?= git-${GIT_SHA}
BINARY_VERSION ?= ${GIT_TAG}

# Only set Version if building a tag or VERSION is set
ifneq ($(BINARY_VERSION),)
	LDFLAGS += -X k8s.io/helm/pkg/version.Version=${BINARY_VERSION}
endif

# Clear the "unreleased" string in BuildMetadata
ifneq ($(GIT_TAG),)
	LDFLAGS += -X k8s.io/helm/pkg/version.BuildMetadata=
endif
LDFLAGS += -X k8s.io/helm/pkg/version.GitCommit=${GIT_COMMIT}
LDFLAGS += -X k8s.io/helm/pkg/version.GitTreeState=${GIT_DIRTY}

IMAGE                := ${DOCKER_REGISTRY}/${IMAGE_PREFIX}/${SHORT_NAME}:${DOCKER_VERSION}
IMAGE_RUDDER         := ${DOCKER_REGISTRY}/${IMAGE_PREFIX}/${SHORT_NAME_RUDDER}:${DOCKER_VERSION}
MUTABLE_IMAGE        := ${DOCKER_REGISTRY}/${IMAGE_PREFIX}/${SHORT_NAME}:${MUTABLE_VERSION}
MUTABLE_IMAGE_RUDDER := ${DOCKER_REGISTRY}/${IMAGE_PREFIX}/${SHORT_NAME_RUDDER}:${DOCKER_VERSION}

DOCKER_PUSH = docker push
ifeq ($(DOCKER_REGISTRY),gcr.io)
	DOCKER_PUSH = gcloud docker push
endif

info:
	 @echo "Version:           ${VERSION}"
	 @echo "Git Tag:           ${GIT_TAG}"
	 @echo "Git Commit:        ${GIT_COMMIT}"
	 @echo "Git Tree State:    ${GIT_DIRTY}"
	 @echo "Docker Version:    ${DOCKER_VERSION}"
	 @echo "Registry:          ${DOCKER_REGISTRY}"
	 @echo "Immutable Image:   ${IMAGE}"
	 @echo "Mutable Image:     ${MUTABLE_IMAGE}"

.PHONY: docker-push
docker-push: docker-mutable-push docker-immutable-push

.PHONY: docker-immutable-push
docker-immutable-push:
	${DOCKER_PUSH} ${IMAGE}

.PHONY: docker-mutable-push
docker-mutable-push:
	${DOCKER_PUSH} ${MUTABLE_IMAGE}
