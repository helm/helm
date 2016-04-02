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
ifeq ($(DOCKER_PROJECT),)
PREFIX := $(DOCKER_REGISTRY)
else
PREFIX := $(DOCKER_REGISTRY)/$(DOCKER_PROJECT)
endif

FULL_IMAGE := $(PREFIX)/$(IMAGE)

TAG ?= git-$(shell git rev-parse --short HEAD)

DEFAULT_PLATFORM := linux
PLATFORM ?= $(DEFAULT_PLATFORM)

DEFAULT_ARCH := amd64
ARCH ?= $(DEFAULT_ARCH)


.PHONY: clean
clean:
	rm -rf bin opt

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
	gcloud docker push $(FULL_IMAGE):latest
else
	docker push $(FULL_IMAGE):$(TAG)
	docker push $(FULL_IMAGE):latest
endif

.PHONY: container
container: .project .docker binary extras
	docker build -t $(FULL_IMAGE):$(TAG) -f Dockerfile .
	docker tag -f $(FULL_IMAGE):$(TAG) $(FULL_IMAGE):latest

.project:
ifeq ($(DOCKER_REGISTRY), gcr.io)
ifeq ($(DOCKER_PROJECT),)
	$(error "Both DOCKER_REGISTRY and DOCKER_PROJECT must be set.")
endif
endif

.docker:
	@if [[ -z `which docker` ]] || ! docker --version &> /dev/null; then echo "docker is not installed correctly"; exit 1; fi

.PHONY: binary
binary:
	@if [[ ! -x "bin/$(IMAGE)" ]] ; then echo "binary bin/$(IMAGE) not found" ; exit 1 ; fi  

.PHONY: kubectl
kubectl:
ifeq ("$(wildcard bin/$(KUBE_VERSION))", "")
	touch bin/$(KUBE_VERSION)
	curl -fsSL -o bin/kubectl https://storage.googleapis.com/kubernetes-release/release/${KUBE_VERSION}/bin/linux/amd64/kubectl
	chmod +x bin/kubectl
endif
