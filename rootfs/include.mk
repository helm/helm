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

# Legacy support for $PROJECT
DOCKER_PROJECT ?= $(PROJECT)

# Support both local and remote repos, and support no project.
ifdef $(DOCKER_PROJECT)
PREFIX := $(DOCKER_REGISTRY)/$(DOCKER_PROJECT)
else
PREFIX := $(DOCKER_REGISTRY)
endif

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
	docker build -t $(FULL_IMAGE):$(TAG) -f Dockerfile .

.project:
ifeq ($(DOCKER_REGISTRY), gcr.io)
ifeq ($(DOCKER_PROJECT),)
	$(error "Both DOCKER_REGISTRY and DOCKER_PROJECT must be set.")
endif
endif

.docker:
	@if [[ -z `which docker` ]] || ! docker --version &> /dev/null; then echo "docker is not installed correctly"; exit 1; fi

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

.PHONY: kubectl
kubectl:
ifeq ("$(wildcard bin/$(KUBE_VERSION))", "")
	touch bin/$(KUBE_VERSION)
	curl -fsSL -o bin/kubectl https://storage.googleapis.com/kubernetes-release/release/${KUBE_VERSION}/bin/linux/amd64/kubectl
	chmod +x bin/kubectl
endif
