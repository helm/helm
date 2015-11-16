# Makefile for the Docker image $(DOCKER_REGISTRY)/$(PROJECT)/expandybird
# MAINTAINER: Jack Greenfield <jackgr@google.com>
# If you update this image please check the tag value before pushing.

.PHONY : all build test push container clean

DOCKER_REGISTRY := gcr.io
PREFIX := $(DOCKER_REGISTRY)/$(PROJECT)
IMAGE := expandybird
TAG := latest

DIR := .

push: container
ifeq ($(DOCKER_REGISTRY),gcr.io)
	gcloud docker push $(PREFIX)/$(IMAGE):$(TAG)
else
	docker push $(PREFIX)/$(IMAGE):$(TAG)
endif

container: expandybird
	cp $(shell which expandybird) .
	docker build -t $(PREFIX)/$(IMAGE):$(TAG) $(DIR)
	rm -f expandybird

expandybird:
	go get -v ./...
	go install -v ./...

clean:
	-docker rmi $(PREFIX)/$(IMAGE):$(TAG)
	rm -f expandybird

