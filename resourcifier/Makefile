# Makefile for the Docker image gcr.io/$(PROJECT)/resourcifier
# MAINTAINER: Jack Greenfield <jackgr@google.com>
# If you update this image please check the tag value before pushing.

.PHONY : all build test push container clean

PREFIX := gcr.io/$(PROJECT)
IMAGE := resourcifier
TAG := latest

ROOT_DIR := $(abspath ./..)
DIR = $(ROOT_DIR)

push: container
	gcloud docker push $(PREFIX)/$(IMAGE):$(TAG)

container:
	docker build -t $(PREFIX)/$(IMAGE):$(TAG) -f Dockerfile $(DIR)

clean:
	-docker rmi $(PREFIX)/$(IMAGE):$(TAG)
