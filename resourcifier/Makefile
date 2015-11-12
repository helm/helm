# Makefile for the Docker image $(DOCKER_REGISTRY)/$(PROJECT)/resourcifier
# MAINTAINER: Jack Greenfield <jackgr@google.com>
# If you update this image please check the tag value before pushing.

.PHONY : all build test push container clean

DOCKER_REGISTRY := gcr.io
PREFIX := $(DOCKER_REGISTRY)/$(PROJECT)
IMAGE := resourcifier
TAG := latest

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
