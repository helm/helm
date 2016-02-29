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

include include.mk

GO_PKGS := $(shell glide nv)

.PHONY: build
build:
	@scripts/build-go.sh

.PHONY: all
all: build

.PHONY: clean
clean:
	go clean -v $(GO_PKGS)

.PHONY: clean
test: build lint vet test-unit

.PHONY: push
push: container

.PHONY: container
container: .project .docker

.PHONY: test-unit
test-unit:
	@echo Running tests...
	go test -v $(shell glide nv)

.PHONY: lint
lint:
	@echo Running golint...
	@for i in $(shell glide nv); do \
		golint $$i; \
	done
	@echo -----------------

.PHONY: vet
vet:
	@echo Running go vet...
	@for i in $(shell glide nv -x); do \
		go tool vet $$i; \
	done
	@echo -----------------

.PHONY: setup-gotools
setup-gotools:
	@echo Installing golint
	go get -u github.com/golang/lint/golint
	@echo Installing vet
	go get -u -v golang.org/x/tools/cmd/vet

.PHONY: .project
.project:
	@if [[ -z "${PROJECT}" ]]; then echo "PROJECT variable must be set"; exit 1; fi

.PHONY: .docker
.docker:
	@if [[ -z `which docker` ]] || ! docker version &> /dev/null; then echo "docker is not installed correctly"; exit 1; fi
