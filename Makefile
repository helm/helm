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

ifndef GOPATH
$(error No GOPATH set)
endif

include include.mk

GO_DIRS ?= $(shell glide nv -x )
GO_PKGS ?= $(shell glide nv)

.PHONY: build
build:
	@scripts/build-go.sh

.PHONY: all
all: build

.PHONY: clean
clean:
	go clean -v $(GO_PKGS)
	rm -rf bin

.PHONY: test
test: build test-style test-unit

.PHONY: push
push: container

.PHONY: container
container: .project .docker

.PHONY: test-unit
test-unit:
	@echo Running tests...
	go test -v $(GO_PKGS)

.PHONY: .test-style
test-style: lint vet
	@if [ $(shell gofmt -e -l -s $(GO_DIRS) | wc -l) ]; then \
		echo "gofmt check failed:"; gofmt -e -d -s $(GO_DIRS); exit 1; \
	fi

.PHONY: lint
lint:
	@echo Running golint...
	@for i in $(GO_PKGS); do \
		golint $$i; \
	done
	@echo -----------------

.PHONY: vet
vet:
	@echo Running go vet...
	@for i in $(GO_DIRS); do \
		go tool vet $$i; \
	done
	@echo -----------------

.PHONY: bootstrap
bootstrap:
	@echo Installing deps
	go get -u github.com/golang/lint/golint
	go get -u golang.org/x/tools/cmd/vet
	go get -u github.com/mitchellh/gox
	glide install

.PHONY: .project
.project:
	@if [[ -z "${PROJECT}" ]]; then echo "PROJECT variable must be set"; exit 1; fi

.PHONY: .docker
.docker:
	@if [[ -z `which docker` ]] || ! docker version &> /dev/null; then echo "docker is not installed correctly"; exit 1; fi

