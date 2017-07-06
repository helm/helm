DOCKER_REGISTRY   ?= gcr.io
IMAGE_PREFIX      ?= kubernetes-helm
SHORT_NAME        ?= tiller
SHORT_NAME_RUDDER ?= rudder
# Helm CLI platforms
TARGETS           = darwin/amd64 linux/amd64 linux/386 linux/arm linux/arm64 linux/ppc64le linux/s390x windows/amd64
# Tiller docker image platforms
ALL_ARCH          = amd64 arm arm64 ppc64le s390x
DIST_DIRS         = find * -type d -exec
APP               = helm

# go option
GO        ?= go
PKG       := $(shell glide novendor)
TAGS      :=
TESTS     := .
TESTFLAGS :=
LDFLAGS   :=
GOFLAGS   :=
BINDIR    := $(CURDIR)/bin
BINARIES  := helm tiller
ARCH      ?= amd64
QEMUVERSION=v2.7.0

# Required for globs to work correctly
SHELL=/bin/bash

ifeq ($(ARCH),amd64)
	BASEIMAGE?=alpine:3.6
endif
ifeq ($(ARCH),arm)
	BASEIMAGE?=arm32v6/alpine:3.6
	QEMUARCH=arm
endif
ifeq ($(ARCH),arm64)
	BASEIMAGE?=arm64v8/alpine:3.6
	QEMUARCH=aarch64
endif
ifeq ($(ARCH),ppc64le)
	BASEIMAGE?=ppc64le/alpine:3.6
	QEMUARCH=ppc64le
endif
ifeq ($(ARCH),s390x)
	BASEIMAGE?=s390x/alpine:3.6
	QEMUARCH=s390x
endif

include versioning.mk

.PHONY: all
all: build

.PHONY: build
build:
	GOBIN=$(BINDIR) $(GO) install $(GOFLAGS) -tags '$(TAGS)' -ldflags '$(LDFLAGS)' k8s.io/helm/cmd/...

# usage: make clean build-cross dist APP=helm|tiller VERSION=v2.0.0-alpha.3
.PHONY: build-cross
build-cross: LDFLAGS += -extldflags "-static"
build-cross:
	CGO_ENABLED=0 gox -output="_dist/{{.OS}}-{{.Arch}}/{{.Dir}}" -osarch='$(TARGETS)' $(GOFLAGS) -tags '$(TAGS)' -ldflags '$(LDFLAGS)' k8s.io/helm/cmd/$(APP)

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
docker-binary-%: BINDIR = ./rootfs
docker-binary-%: GOFLAGS += -a -installsuffix cgo
docker-binary-%:
	docker run -it -v $(shell pwd)/$(BINDIR):/build -v $(shell pwd):/go/src/k8s.io/helm -e GOARCH=$(ARCH) golang:1.8 /bin/bash -c "\
		CGO_ENABLED=0 go build -o /build/$* $(GOFLAGS) -tags '$(TAGS)' -ldflags '$(LDFLAGS)' k8s.io/helm/cmd/$*"

docker-build-prepare-%:
	cp rootfs/Dockerfile.$* rootfs/Dockerfile.$*.$(ARCH)
	sed -i "s|QEMUARCH|$(QEMUARCH)|g" rootfs/Dockerfile.$*.$(ARCH)
	sed -i "s|BASEIMAGE|$(BASEIMAGE)|g" rootfs/Dockerfile.$*.$(ARCH)
ifeq ($(ARCH),amd64)
	# When building "normally", remove the whole line, it has no part in the image
	sed -i "/CROSS_BUILD_/d" rootfs/Dockerfile.$*.$(ARCH)
else
	sed -i "s/CROSS_BUILD_//g" rootfs/Dockerfile.$*.$(ARCH)

	# When cross-building, only the placeholder "CROSS_BUILD_" should be removed
	# Register /usr/bin/qemu-ARCH-static as the handler for ARM binaries in the kernel
	docker run --rm --privileged multiarch/qemu-user-static:register --reset
	curl -sSL --retry 5 https://github.com/multiarch/qemu-user-static/releases/download/$(QEMUVERSION)/x86_64_qemu-$(QEMUARCH)-static.tar.gz | tar -xz -C rootfs
endif

.PHONY: docker-build
docker-build: check-docker docker-binary-tiller docker-build-prepare-tiller
	docker build --rm -t $(IMAGE) -f rootfs/Dockerfile.tiller.$(ARCH) rootfs
	docker tag $(IMAGE) $(MUTABLE_IMAGE)
	rm rootfs/Dockerfile.tiller.$(ARCH)

.PHONY: docker-build-experimental
docker-build-experimental: check-docker docker-binary-tiller docker-binary-rudder docker-build-prepare-experimental docker-build-prepare-rudder
	docker build --rm -t ${IMAGE} -f rootfs/Dockerfile.experimental.$(ARCH) rootfs
	docker tag ${IMAGE} ${MUTABLE_IMAGE}
	docker build --rm -t ${IMAGE_RUDDER} -f rootfs/Dockerfile.rudder.$(ARCH) rootfs
	docker tag ${IMAGE_RUDDER} ${MUTABLE_IMAGE_RUDDER}
	rm rootfs/Dockerfile.experimental.$(ARCH) rootfs/Dockerfile.rudder.$(ARCH)

.PHONY: test
test: build
test: TESTFLAGS += -race -v
test: test-style
test: test-unit

.PHONY: test-unit
test-unit:
	@echo
	@echo "==> Running unit tests <=="
	HELM_HOME=/no/such/dir $(GO) test $(GOFLAGS) -run $(TESTS) $(PKG) $(TESTFLAGS)

.PHONY: test-style
test-style:
	@scripts/validate-go.sh
	@scripts/validate-license.sh

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
HAS_HG := $(shell command -v hg;)

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
ifndef HAS_HG
	$(error You must install Mercurial)
endif
	glide install --strip-vendor
	go build -o bin/protoc-gen-go ./vendor/github.com/golang/protobuf/protoc-gen-go
	scripts/setup-apimachinery.sh

bootstrap-dockerized:
	docker run -it -v $(shell pwd):/go/src/k8s.io/helm -w /go/src/k8s.io/helm gcr.io/google_containers/kube-cross:v1.8.3-1 /bin/bash -c "make bootstrap"
