# Copyright 2016 The Kubernetes Authors All rights reserved.
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
DOCKER_REGISTRY ?= gcr.io

# Legacy support for $PROJECT
DOCKER_PROJECT ?= $(PROJECT)

# Support both local and remote repos, and support no project.
ifdef $(DOCKER_PROJECT)
PREFIX := $(DOCKER_REGISTRY)/$(DOCKER_PROJECT)
else
PREFIX := $(DOCKER_REGISTRY)
endif

TAG ?= git-$(shell git rev-parse --short HEAD)

ROOT_DIR := $(abspath ./..)
kubectl:
ifeq ("$(wildcard bin/$(KUBE_VERSION))", "")
	touch bin/$(KUBE_VERSION)
	curl -fsSL -o bin/kubectl https://storage.googleapis.com/kubernetes-release/release/${KUBE_VERSION}/bin/linux/amd64/kubectl
	chmod +x bin/kubectl
endif
