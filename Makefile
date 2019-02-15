BINDIR     := $(CURDIR)/bin
DIST_DIRS  := find * -type d -exec
TARGETS    := darwin/amd64 linux/amd64 linux/386 linux/arm linux/arm64 linux/ppc64le windows/amd64
BINNAME    ?= helm

GOPATH     = $(shell go env GOPATH)
DEP        = $(GOPATH)/bin/dep
GOX        = $(GOPATH)/bin/gox
GOIMPORTS  = $(GOPATH)/bin/goimports

# go option
PKG        := ./...
TAGS       :=
TESTS      := .
TESTFLAGS  :=
LDFLAGS    := -w -s
GOFLAGS    :=
SRC        := $(shell find . -type f -name '*.go' -print)

# Required for globs to work correctly
SHELL      = /bin/bash

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
	LDFLAGS += -X k8s.io/helm/internal/version.version=${BINARY_VERSION}
endif

# Clear the "unreleased" string in BuildMetadata
ifneq ($(GIT_TAG),)
	LDFLAGS += -X k8s.io/helm/internal/version.metadata=
endif
LDFLAGS += -X k8s.io/helm/internal/version.gitCommit=${GIT_COMMIT}
LDFLAGS += -X k8s.io/helm/internal/version.gitTreeState=${GIT_DIRTY}

.PHONY: all
all: build

# ------------------------------------------------------------------------------
#  build

.PHONY: build
build: $(BINDIR)/$(BINNAME)

$(BINDIR)/$(BINNAME): $(SRC) vendor
	go build $(GOFLAGS) -tags '$(TAGS)' -ldflags '$(LDFLAGS)' -o $(BINDIR)/$(BINNAME) k8s.io/helm/cmd/helm

# ------------------------------------------------------------------------------
#  test

.PHONY: test
test: build
test: TESTFLAGS += -race -v
test: test-style
test: test-unit

.PHONY: test-unit
test-unit: vendor
	@echo
	@echo "==> Running unit tests <=="
	HELM_HOME=/no_such_dir go test $(GOFLAGS) -run $(TESTS) $(PKG) $(TESTFLAGS)

.PHONY: test-style
test-style: vendor
	@scripts/validate-go.sh
	@scripts/validate-license.sh

.PHONY: verify-docs
verify-docs: build
	@scripts/verify-docs.sh

.PHONY: coverage
coverage:
	@scripts/coverage.sh

.PHONY: format
format: $(GOIMPORTS)
	go list -f '{{.Dir}}' ./... | xargs $(GOIMPORTS) -w -local k8s.io/helm

# ------------------------------------------------------------------------------
#  dependencies

.PHONY: bootstrap
bootstrap: vendor

$(DEP):
	go get -u github.com/golang/dep/cmd/dep

$(GOX):
	go get -u github.com/mitchellh/gox

$(GOIMPORTS):
	go get -u golang.org/x/tools/cmd/goimports

# install vendored dependencies
vendor: Gopkg.lock
	$(DEP) ensure -v --vendor-only

# update vendored dependencies
Gopkg.lock: Gopkg.toml
	$(DEP) ensure -v --no-vendor

Gopkg.toml: $(DEP)

# ------------------------------------------------------------------------------
#  release

.PHONY: build-cross
build-cross: LDFLAGS += -extldflags "-static"
build-cross: vendor
build-cross: $(GOX)
	CGO_ENABLED=0 $(GOX) -parallel=3 -output="_dist/{{.OS}}-{{.Arch}}/$(BINNAME)" -osarch='$(TARGETS)' $(GOFLAGS) -tags '$(TAGS)' -ldflags '$(LDFLAGS)' k8s.io/helm/cmd/helm

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

# ------------------------------------------------------------------------------

.PHONY: docs
docs: build
	@scripts/update-docs.sh

.PHONY: clean
clean:
	@rm -rf $(BINDIR) ./_dist

.PHONY: info
info:
	 @echo "Version:           ${VERSION}"
	 @echo "Git Tag:           ${GIT_TAG}"
	 @echo "Git Commit:        ${GIT_COMMIT}"
	 @echo "Git Tree State:    ${GIT_DIRTY}"
