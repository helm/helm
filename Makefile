BINDIR      := $(CURDIR)/bin
INSTALL_PATH ?= /usr/local/bin
DIST_DIRS   := find * -type d -exec
TARGETS     := darwin/amd64 darwin/arm64 linux/amd64 linux/386 linux/arm linux/arm64 linux/loong64 linux/ppc64le linux/s390x linux/riscv64 windows/amd64 windows/arm64
TARGET_OBJS ?= darwin-amd64.tar.gz darwin-amd64.tar.gz.sha256 darwin-amd64.tar.gz.sha256sum darwin-arm64.tar.gz darwin-arm64.tar.gz.sha256 darwin-arm64.tar.gz.sha256sum linux-amd64.tar.gz linux-amd64.tar.gz.sha256 linux-amd64.tar.gz.sha256sum linux-386.tar.gz linux-386.tar.gz.sha256 linux-386.tar.gz.sha256sum linux-arm.tar.gz linux-arm.tar.gz.sha256 linux-arm.tar.gz.sha256sum linux-arm64.tar.gz linux-arm64.tar.gz.sha256 linux-arm64.tar.gz.sha256sum linux-loong64.tar.gz linux-loong64.tar.gz.sha256 linux-loong64.tar.gz.sha256sum linux-ppc64le.tar.gz linux-ppc64le.tar.gz.sha256 linux-ppc64le.tar.gz.sha256sum linux-s390x.tar.gz linux-s390x.tar.gz.sha256 linux-s390x.tar.gz.sha256sum linux-riscv64.tar.gz linux-riscv64.tar.gz.sha256 linux-riscv64.tar.gz.sha256sum windows-amd64.zip windows-amd64.zip.sha256 windows-amd64.zip.sha256sum windows-arm64.zip windows-arm64.zip.sha256 windows-arm64.zip.sha256sum
BINNAME     ?= helm

GOBIN         = $(shell go env GOBIN)
ifeq ($(GOBIN),)
GOBIN         = $(shell go env GOPATH)/bin
endif
GORELEASER    = $(GOBIN)/goreleaser
GOIMPORTS     = $(GOBIN)/goimports
ARCH          = $(shell go env GOARCH)

ACCEPTANCE_DIR := ../acceptance-testing
# To specify the subset of acceptance tests to run. '.' means all tests
ACCEPTANCE_RUN_TESTS = .

# go option
PKG         := ./...
TAGS        :=
TESTS       := .
TESTFLAGS   := -shuffle=on -count=1
LDFLAGS     := -w -s
GOFLAGS     :=
CGO_ENABLED ?= 0

# Rebuild the binary if any of these files change
SRC := $(shell find . -type f -name '*.go' -print) go.mod go.sum

# Required for globs to work correctly
SHELL      = /usr/bin/env bash

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
	LDFLAGS += -X helm.sh/helm/v4/internal/version.version=${BINARY_VERSION}
endif

VERSION_METADATA = unreleased
# Clear the "unreleased" string in BuildMetadata
ifneq ($(GIT_TAG),)
	VERSION_METADATA =
endif

LDFLAGS += -X helm.sh/helm/v4/internal/version.metadata=${VERSION_METADATA}
LDFLAGS += -X helm.sh/helm/v4/internal/version.gitCommit=${GIT_COMMIT}
LDFLAGS += -X helm.sh/helm/v4/internal/version.gitTreeState=${GIT_DIRTY}
LDFLAGS += $(EXT_LDFLAGS)

# Define constants based on the client-go version
K8S_MODULES_VER=$(subst ., ,$(subst v,,$(shell go list -f '{{.Version}}' -m k8s.io/client-go)))
K8S_MODULES_MAJOR_VER=$(shell echo $$(($(firstword $(K8S_MODULES_VER)) + 1)))
K8S_MODULES_MINOR_VER=$(word 2,$(K8S_MODULES_VER))

LDFLAGS += -X helm.sh/helm/v4/pkg/chart/v2/lint/rules.k8sVersionMajor=$(K8S_MODULES_MAJOR_VER)
LDFLAGS += -X helm.sh/helm/v4/pkg/chart/v2/lint/rules.k8sVersionMinor=$(K8S_MODULES_MINOR_VER)
LDFLAGS += -X helm.sh/helm/v4/pkg/internal/v3/lint/rules.k8sVersionMajor=$(K8S_MODULES_MAJOR_VER)
LDFLAGS += -X helm.sh/helm/v4/pkg/internal/v3/lint/rules.k8sVersionMinor=$(K8S_MODULES_MINOR_VER)
LDFLAGS += -X helm.sh/helm/v4/pkg/chart/common/util.k8sVersionMajor=$(K8S_MODULES_MAJOR_VER)
LDFLAGS += -X helm.sh/helm/v4/pkg/chart/common/util.k8sVersionMinor=$(K8S_MODULES_MINOR_VER)
LDFLAGS += -X helm.sh/helm/v4/internal/version.kubeClientVersionMajor=$(K8S_MODULES_MAJOR_VER)
LDFLAGS += -X helm.sh/helm/v4/internal/version.kubeClientVersionMinor=$(K8S_MODULES_MINOR_VER)

.PHONY: all
all: build

# ------------------------------------------------------------------------------
#  build

.PHONY: build
build: $(BINDIR)/$(BINNAME) tidy

$(BINDIR)/$(BINNAME): $(SRC)
	CGO_ENABLED=$(CGO_ENABLED) go build $(GOFLAGS) -trimpath -tags '$(TAGS)' -ldflags '$(LDFLAGS)' -o '$(BINDIR)'/$(BINNAME) ./cmd/helm

# ------------------------------------------------------------------------------
#  install

.PHONY: install
install: build
	@install "$(BINDIR)/$(BINNAME)" "$(INSTALL_PATH)/$(BINNAME)"

# ------------------------------------------------------------------------------
#  test

.PHONY: test
test: build
ifeq ($(ARCH),s390x)
test: TESTFLAGS += -v
else
test: TESTFLAGS += -race -v
endif
test: test-style
test: test-unit

.PHONY: test-unit
test-unit:
	@echo
	@echo "==> Running unit tests <=="
	go test $(GOFLAGS) -run $(TESTS) $(PKG) $(TESTFLAGS)
	@echo
	@echo "==> Running unit test(s) with ldflags <=="
# Test to check the deprecation warnings on Kubernetes templates created by `helm create` against the current Kubernetes
# version. Note: The version details are set in var LDFLAGS. To avoid the ldflags impact on other unit tests that are
# based on older versions, this is run separately. When run without the ldflags in the unit test (above) or coverage
# test, it still passes with a false-positive result as the resources shouldn’t be deprecated in the older Kubernetes
# version if it only starts failing with the latest.
	go test $(GOFLAGS) -run ^TestHelmCreateChart_CheckDeprecatedWarnings$$ ./pkg/chart/v2/lint/ $(TESTFLAGS) -ldflags '$(LDFLAGS)'
	go test $(GOFLAGS) -run ^TestHelmCreateChart_CheckDeprecatedWarnings$$ ./internal/chart/v3/lint/ $(TESTFLAGS) -ldflags '$(LDFLAGS)'


# To run the coverage for a specific package use: make test-coverage PKG=./pkg/action
.PHONY: test-coverage
test-coverage:
	@echo
	@echo "==> Running unit tests with coverage: $(PKG) <=="
	@ ./scripts/coverage.sh $(PKG)

.PHONY: test-style
test-style:
	golangci-lint run ./...
	@scripts/validate-license.sh

.PHONY: test-source-headers
test-source-headers:
	@scripts/validate-license.sh

.PHONY: test-acceptance
test-acceptance: TARGETS = linux/amd64
test-acceptance: build build-cross
	@if [ -d "${ACCEPTANCE_DIR}" ]; then \
		cd ${ACCEPTANCE_DIR} && \
			ROBOT_RUN_TESTS=$(ACCEPTANCE_RUN_TESTS) ROBOT_HELM_PATH='$(BINDIR)' make acceptance; \
	else \
		echo "You must clone the acceptance_testing repo under $(ACCEPTANCE_DIR)"; \
		echo "You can find the acceptance_testing repo at https://github.com/helm/acceptance-testing"; \
	fi

.PHONY: test-acceptance-completion
test-acceptance-completion: ACCEPTANCE_RUN_TESTS = shells.robot
test-acceptance-completion: test-acceptance

.PHONY: format
format: $(GOIMPORTS)
	go list -f '{{.Dir}}' ./... | xargs $(GOIMPORTS) -w -local helm.sh/helm

# Generate golden files used in unit tests
.PHONY: gen-test-golden
gen-test-golden:
gen-test-golden: PKG = ./pkg/cmd ./pkg/action
gen-test-golden: TESTFLAGS = -update
gen-test-golden: test-unit

# ------------------------------------------------------------------------------
#  dependencies

# If go install is run from inside the project directory it will add the
# dependencies to the go.mod file. To avoid that we change to a directory
# without a go.mod file when downloading the following dependencies

$(GORELEASER):
	echo go install github.com/goreleaser/goreleaser/v2@latest
	(cd /; go install github.com/goreleaser/goreleaser/v2@latest)

$(GOIMPORTS):
	(cd /; go install golang.org/x/tools/cmd/goimports@latest)

# ------------------------------------------------------------------------------
#  release

.PHONY: build-cross
build-cross: LDFLAGS += -extldflags "-static"
build-cross: $(GORELEASER)
	LDFLAGS='$(LDFLAGS)' $(GORELEASER) build --snapshot --clean

.PHONY: dist
dist:
	LDFLAGS='$(LDFLAGS)' $(GORELEASER) release --snapshot --clean

.PHONY: fetch-dist
fetch-dist:
	mkdir -p _dist
	cd _dist && \
	for obj in ${TARGET_OBJS} ; do \
		curl -sSL -o helm-${VERSION}-$${obj} https://get.helm.sh/helm-${VERSION}-$${obj} ; \
	done

.PHONY: sign
sign:
	for f in $$(ls _dist/*.{gz,zip,sha256,sha256sum} 2>/dev/null) ; do \
		gpg --armor --detach-sign $${f} ; \
	done

# The contents of the .sha256sum file are compatible with tools like
# shasum. For example, using the following command will verify
# the file helm-3.1.0-rc.1-darwin-amd64.tar.gz:
#   shasum -a 256 -c helm-3.1.0-rc.1-darwin-amd64.tar.gz.sha256sum
# The .sha256 files hold only the hash and are not compatible with
# verification tools like shasum or sha256sum. This method and file can be
# removed in Helm v4.
.PHONY: checksum
checksum:
	for f in $$(ls _dist/*.{gz,zip} 2>/dev/null) ; do \
		shasum -a 256 "$${f}" | sed 's/_dist\///' > "$${f}.sha256sum" ; \
		shasum -a 256 "$${f}" | awk '{print $$1}' > "$${f}.sha256" ; \
	done

# ------------------------------------------------------------------------------

.PHONY: clean
clean:
	@rm -rf '$(BINDIR)' ./_dist

.PHONY: release-notes
release-notes:
	@if [ ! -d "./_dist" ]; then \
		echo "please run 'make fetch-dist' first" && \
		exit 1; \
	fi
	@if [ -z "${PREVIOUS_RELEASE}" ]; then \
		echo "please set PREVIOUS_RELEASE environment variable" && \
		exit 1; \
	fi
	@./scripts/release-notes.sh ${PREVIOUS_RELEASE} ${VERSION}

.PHONY: info
info:
	@echo "Version:           ${VERSION}"
	@echo "Git Tag:           ${GIT_TAG}"
	@echo "Git Commit:        ${GIT_COMMIT}"
	@echo "Git Tree State:    ${GIT_DIRTY}"

.PHONY: tidy
tidy:
	go mod tidy
