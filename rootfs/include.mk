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

# If you update this image please check the tag value before pushing.

DOCKER_REGISTRY ?= gcr.io
PREFIX := $(DOCKER_REGISTRY)/$(PROJECT)
FULL_IMAGE := $(PREFIX)/$(IMAGE)

TAG ?= git-$(shell git rev-parse --short HEAD)

DEFAULT_PLATFORM := $(shell uname | tr '[:upper:]' '[:lower:]')
PLATFORM ?= $(DEFAULT_PLATFORM)

DEFAULT_ARCH := $(shell uname -m)
ARCH ?= $(DEFAULT_ARCH)

.PHONY: info
info:
	@echo "Build tag: ${TAG}"
	@echo "Registry: ${DOCKER_REGISTRY}"
	@echo "Project: ${PROJECT}"
	@echo "Image: ${IMAGE}"
	@echo "Platform: ${PLATFORM}"
	@echo "Arch: ${ARCH}"

.PHONY : .project

.PHONY: push
push: container
ifeq ($(DOCKER_REGISTRY),gcr.io)
	gcloud docker push $(FULL_IMAGE):$(TAG)
else
	docker push $(FULL_IMAGE):$(TAG)
endif

.PHONY: container
container: .project .docker binary extras
	docker build -t $(FULL_IMAGE):latest -f Dockerfile .
	docker tag -f $(FULL_IMAGE):latest $(FULL_IMAGE):$(TAG)

.project:
	@if [[ -z "${PROJECT}" ]]; then echo "PROJECT variable must be set"; exit 1; fi

.docker:
	@if [[ -z `which docker` ]] || ! docker version &> /dev/null; then echo "docker is not installed correctly"; exit 1; fi

CROSS_IMAGE := $(PLATFORM)-$(ARCH)/$(IMAGE)/$(IMAGE)

.PHONY: binary
binary:
	@if [[ -z $(CROSS_IMAGE) ]]; then \
		echo cp ../../bin/$(CROSS_IMAGE) ./bin ; \
		cp ../../bin/$(CROSS_IMAGE) ./bin ; \
	else \
		echo cp ../../bin/$(IMAGE) ./bin ; \
		cp ../../bin/$(IMAGE) ./bin ; \
	fi
