DOCKER_REGISTRY ?= gcr.io
IMAGE_PREFIX    ?= deis-sandbox
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
BINDIR    := ./bin
BINARIES  := helm tiller

.PHONY: all
all: build

.PHONY: build
build: GOFLAGS += -i
build:
	@for i in $(BINARIES); do \
		CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) $(GO) build -o $(BINDIR)/$$i $(GOFLAGS) -tags '$(TAGS)' -ldflags '$(LDFLAGS)' ./cmd/$$i || exit 1; \
	done

.PHONY: check-docker
check-docker:
	@if [ -z $$(which docker) ]; then \
	  echo "Missing \`docker\` client which is required for development"; \
	  exit 2; \
	fi

.PHONY: docker-binary
docker-binary: GOOS = linux
docker-binary: GOARCH = amd64
docker-binary: BINDIR = ./rootfs
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
