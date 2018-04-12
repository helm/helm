GIT_COMMIT = $(shell git rev-parse HEAD)
GIT_SHA    = $(shell git rev-parse --short HEAD)
GIT_TAG    = $(shell git describe --tags --abbrev=0 --exact-match 2>/dev/null)
GIT_DIRTY  = $(shell test -n "`git status --porcelain`" && echo "dirty" || echo "clean")

ifdef VERSION
	BINARY_VERSION = $(VERSION)
endif

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

info:
	 @echo "Version:           ${VERSION}"
	 @echo "Git Tag:           ${GIT_TAG}"
	 @echo "Git Commit:        ${GIT_COMMIT}"
	 @echo "Git Tree State:    ${GIT_DIRTY}"
