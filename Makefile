# Copyright 2015 The Kubernetes Authors All rights reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

.PHONY: info
info:
	$(MAKE) -C $(ROOTFS) $@

.PHONY: gocheck
ifndef GOPATH
	$(error No GOPATH set)
endif

GO_DIRS ?= $(shell glide nv -x )
GO_PKGS ?= $(shell glide nv)

.PHONY: build
build: gocheck
	@scripts/build-go.sh

.PHONY: build-cross
build-cross: gocheck
	@BUILD_CROSS=1 scripts/build-go.sh

.PHONY: all
all: build

.PHONY: clean
clean:
	go clean -v $(GO_PKGS)
	rm -rf bin

.PHONY: test
test: build test-style test-unit test-flake8

ROOTFS := rootfs

.PHONY: push
push: all
	$(MAKE) -C $(ROOTFS) $@

.PHONY: container
container: all
	$(MAKE) -C $(ROOTFS) $@

.PHONY: test-unit
test-unit:
	@echo Running tests...
	go test -v $(GO_PKGS)

.PHONY: test-flake8
test-flake8:
	@echo Running flake8...
	flake8 expansion
	@echo ----------------

.PHONY: test-style
test-style:
	@scripts/validate-go.sh

HAS_GLIDE := $(shell command -v glide;)
HAS_GOLINT := $(shell command -v golint;)
HAS_GOVET := $(shell command -v go tool vet;)
HAS_GOX := $(shell command -v gox;)

.PHONY: bootstrap
bootstrap:
	@echo Installing deps
ifndef HAS_GLIDE
	go get github.com/Masterminds/glide
endif
ifndef HAS_GOLINT
	go get -u github.com/golang/lint/golint
endif
ifndef HAS_GOVET
	go get -u golang.org/x/tools/cmd/vet
endif
ifndef HAS_GOX
	go get -u github.com/mitchellh/gox
endif
	glide install
