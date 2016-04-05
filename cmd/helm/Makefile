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

SHELL := /bin/bash

GOLANG_CROSSPLATFORMS := darwin/386 darwin/amd64 freebsd/386 freebsd/amd64 freebsd/arm linux/386 linux/amd64 linux/arm windows/386 windows/amd64

ROOT_DIR := $(abspath ./../..)
BIN_DIR := $(ROOT_DIR)/bin

DEFAULT_BUCKET := gs://get-helm
STORAGE_BUCKET ?= $(DEFAULT_BUCKET)

DEFAULT_TAG := git-$(shell git rev-parse --short HEAD)
TAG ?= $(DEFAULT_TAG)

all: push

push:
	for platform in ${GOLANG_CROSSPLATFORMS}; do \
		echo $$platform; \
		PLATFORM=$${platform%/*} && ARCH=$${platform##*/} && \
		BINARY=$${PLATFORM}-$${ARCH} && \
		ZIP=${TAG}-helm-$${BINARY}.zip && \
		zip -j $${ZIP} ${BIN_DIR}/$${BINARY}/helm* && \
		gsutil cp $${ZIP} ${STORAGE_BUCKET} && \
		rm $${ZIP} ; \
	done
