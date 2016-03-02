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

include ../include.mk

.PHONY : all build push container clean .project

DOCKER_REGISTRY := gcr.io
PREFIX := $(DOCKER_REGISTRY)/$(PROJECT)
IMAGE := expandybird

ROOT_DIR := $(abspath ./..)
DIR = $(ROOT_DIR)

push: container
ifeq ($(DOCKER_REGISTRY),gcr.io)
	gcloud docker push $(PREFIX)/$(IMAGE):$(TAG)
else
	docker push $(PREFIX)/$(IMAGE):$(TAG)
endif

container:
	docker build -t $(PREFIX)/$(IMAGE):$(TAG) -f Dockerfile $(DIR)

clean:
	-docker rmi $(PREFIX)/$(IMAGE):$(TAG)

.PHONY: test
test: lint vet test-unit
