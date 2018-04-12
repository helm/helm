TARGETS           ?= darwin/amd64 linux/amd64 linux/386 linux/arm linux/arm64 linux/ppc64le windows/amd64
DIST_DIRS         = find * -type d -exec
APP               = helm

# go option
GO        ?= go
PKG       := $(shell glide novendor)
TAGS      :=
TESTS     := .
TESTFLAGS :=
LDFLAGS   := -w -s
GOFLAGS   :=
BINDIR    := $(CURDIR)/bin
BINARIES  := helm

# Required for globs to work correctly
SHELL=/bin/bash

.PHONY: all
all: build

.PHONY: build
build:
	GOBIN=$(BINDIR) $(GO) install $(GOFLAGS) -tags '$(TAGS)' -ldflags '$(LDFLAGS)' k8s.io/helm/cmd/...

# usage: make clean build-cross dist APP=helm|tiller VERSION=v2.0.0-alpha.3
.PHONY: build-cross
build-cross: LDFLAGS += -extldflags "-static"
build-cross:
	CGO_ENABLED=0 gox -parallel=3 -output="_dist/{{.OS}}-{{.Arch}}/{{.Dir}}" -osarch='$(TARGETS)' $(GOFLAGS) -tags '$(TAGS)' -ldflags '$(LDFLAGS)' k8s.io/helm/cmd/$(APP)

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
	@rm -rf $(BINDIR) ./_dist

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
