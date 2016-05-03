DOCKER_REGISTRY ?= gcr.io
IMAGE_PREFIX    ?= kubernetes-helm
SHORT_NAME      ?= tiller

# go option
GO        ?= go
GOARCH    ?= $(shell go env GOARCH)
GOOS      ?= $(shell go env GOOS)
PKG       := $(shell glide novendor)
TAGS      :=
TESTS     := .
TESTFLAGS :=
LDFLAGS   :=
GOFLAGS   :=
BINDIR    := $(CURDIR)/bin
BINARIES  := helm tiller

.PHONY: all
all: build

.PHONY: build
docker-binary: GOFLAGS += -i
build:
	GOBIN=$(BINDIR) $(GO) install $(GOFLAGS) -tags '$(TAGS)' -ldflags '$(LDFLAGS)' github.com/kubernetes/helm/cmd/...

.PHONY: check-docker
check-docker:
	@if [ -z $$(which docker) ]; then \
	  echo "Missing \`docker\` client which is required for development"; \
	  exit 2; \
	fi

.PHONY: docker-binary
docker-binary: GOOS = linux
docker-binary: GOARCH = amd64
docker-binary: BINDIR = $(CURDIR)/rootfs
docker-binary: GOFLAGS += -a -installsuffix cgo
docker-binary: build

.PHONY: docker-build
docker-build: check-docker docker-binary
	docker build --rm -t ${IMAGE} rootfs
	docker tag -f ${IMAGE} ${MUTABLE_IMAGE}

.PHONY: test
test: build
test: TESTFLAGS += -race -v
test: test-style
test: test-unit

.PHONY: test-unit
test-unit:
	$(GO) test $(GOFLAGS) -run $(TESTS) $(PKG) $(TESTFLAGS)

.PHONY: test-style
test-style:
	@scripts/validate-go.sh

.PHONY: clean
clean:
	@rm -rf $(BINDIR)

.PHONY: coverage
coverage:
	@scripts/coverage.sh

.PHONY: bootstrap
bootstrap:
	glide install

include versioning.mk
