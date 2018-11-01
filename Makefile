DOCKER_REGISTRY   ?= gcr.io
IMAGE_PREFIX      ?= kubernetes-helm
DEV_IMAGE         ?= golang:1.11
SHORT_NAME        ?= tiller
SHORT_NAME_RUDDER ?= rudder
TARGETS           ?= darwin/amd64 linux/amd64 linux/386 linux/arm linux/arm64 linux/ppc64le linux/s390x windows/amd64
DIST_DIRS         = find * -type d -exec

# go option
GO        ?= go
PKG       := $(shell glide novendor)
TAGS      :=
TESTS     := .
TESTFLAGS :=
LDFLAGS   := -w -s
GOFLAGS   :=
BINDIR    := $(CURDIR)/bin
BINARIES  := helm tiller

# Required for globs to work correctly
SHELL=/usr/bin/env bash

.PHONY: all
all: build

.PHONY: build
build:
	GOBIN=$(BINDIR) $(GO) install $(GOFLAGS) -tags '$(TAGS)' -ldflags '$(LDFLAGS)' k8s.io/helm/cmd/...

# usage: make clean build-cross dist VERSION=v2.0.0-alpha.3
.PHONY: build-cross
build-cross: LDFLAGS += -extldflags "-static"
build-cross:
	CGO_ENABLED=0 gox -parallel=3 -output="_dist/{{.OS}}-{{.Arch}}/{{.Dir}}" -osarch='$(TARGETS)' $(GOFLAGS) $(if $(TAGS),-tags '$(TAGS)',) -ldflags '$(LDFLAGS)' k8s.io/helm/cmd/helm
	CGO_ENABLED=0 gox -parallel=3 -output="_dist/{{.OS}}-{{.Arch}}/{{.Dir}}" -osarch='$(TARGETS)' $(GOFLAGS) $(if $(TAGS),-tags '$(TAGS)',) -ldflags '$(LDFLAGS)' k8s.io/helm/cmd/tiller

.PHONY: dist
dist:
	( \
		cd _dist && \
		$(DIST_DIRS) cp ../LICENSE {} \; && \
		$(DIST_DIRS) cp ../README.md {} \; && \
		$(DIST_DIRS) tar -zcf helm-${VERSION}-{}.tar.gz {} \; && \
		$(DIST_DIRS) zip -r helm-${VERSION}-{}.zip {} \; \
	)

.PHONY: checksum
checksum:
	for f in _dist/*.{gz,zip} ; do \
		shasum -a 256 "$${f}"  | awk '{print $$1}' > "$${f}.sha256" ; \
	done

.PHONY: check-docker
check-docker:
	@if [ -z $$(which docker) ]; then \
	  echo "Missing \`docker\` client which is required for development"; \
	  exit 2; \
	fi

.PHONY: docker-binary
docker-binary: BINDIR = ./rootfs
docker-binary: GOFLAGS += -a -installsuffix cgo
docker-binary:
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 $(GO) build -o $(BINDIR)/helm $(GOFLAGS) -tags '$(TAGS)' -ldflags '$(LDFLAGS)' k8s.io/helm/cmd/helm
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 $(GO) build -o $(BINDIR)/tiller $(GOFLAGS) -tags '$(TAGS)' -ldflags '$(LDFLAGS)' k8s.io/helm/cmd/tiller

.PHONY: docker-build
docker-build: check-docker docker-binary
	docker build --rm -t ${IMAGE} rootfs
	docker tag ${IMAGE} ${MUTABLE_IMAGE}

.PHONY: docker-binary-rudder
docker-binary-rudder: BINDIR = ./rootfs
docker-binary-rudder: GOFLAGS += -a -installsuffix cgo
docker-binary-rudder:
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 $(GO) build -o $(BINDIR)/rudder $(GOFLAGS) -tags '$(TAGS)' -ldflags '$(LDFLAGS)' k8s.io/helm/cmd/rudder

.PHONY: docker-build-experimental
docker-build-experimental: check-docker docker-binary docker-binary-rudder
	docker build --rm -t ${IMAGE} rootfs -f rootfs/Dockerfile.experimental
	docker tag ${IMAGE} ${MUTABLE_IMAGE}
	docker build --rm -t ${IMAGE_RUDDER} rootfs -f rootfs/Dockerfile.rudder
	docker tag ${IMAGE_RUDDER} ${MUTABLE_IMAGE_RUDDER}

.PHONY: test
test: build
test: TESTFLAGS += -race -v
test: test-style
test: test-unit

.PHONY: docker-test
docker-test: docker-binary
docker-test: TESTFLAGS += -race -v
docker-test: docker-test-style
docker-test: docker-test-unit

.PHONY: test-unit
test-unit:
	@echo
	@echo "==> Running unit tests <=="
	HELM_HOME=/no/such/dir $(GO) test $(GOFLAGS) -run $(TESTS) $(PKG) $(TESTFLAGS)

.PHONY: docker-test-unit
docker-test-unit: check-docker
	docker run \
		-v $(shell pwd):/go/src/k8s.io/helm \
		-w /go/src/k8s.io/helm \
		$(DEV_IMAGE) \
		bash -c "HELM_HOME=/no/such/dir go test $(GOFLAGS) -run $(TESTS) $(PKG) $(TESTFLAGS)"

.PHONY: test-style
test-style:
	@scripts/validate-go.sh
	@scripts/validate-license.sh

.PHONY: docker-test-style
docker-test-style: check-docker
	docker run \
		-v $(CURDIR):/go/src/k8s.io/helm \
		-w /go/src/k8s.io/helm \
		$(DEV_IMAGE) \
		bash -c "scripts/validate-go.sh && scripts/validate-license.sh"

.PHONY: protoc
protoc:
	$(MAKE) -C _proto/ all

.PHONY: docs
docs: build
	@scripts/update-docs.sh

.PHONY: verify-docs
verify-docs: build
	@scripts/verify-docs.sh

.PHONY: clean
clean:
	@rm -rf $(BINDIR) ./rootfs/tiller ./_dist

.PHONY: coverage
coverage:
	@scripts/coverage.sh

HAS_GLIDE := $(shell command -v glide;)
HAS_GOX := $(shell command -v gox;)
HAS_GIT := $(shell command -v git;)

.PHONY: bootstrap
bootstrap:
ifndef HAS_GLIDE
	go get -u github.com/Masterminds/glide
endif
ifndef HAS_GOX
	go get -u github.com/mitchellh/gox
endif

ifndef HAS_GIT
	$(error You must install Git)
endif
	glide install --strip-vendor
	go build -o bin/protoc-gen-go ./vendor/github.com/golang/protobuf/protoc-gen-go

include versioning.mk
