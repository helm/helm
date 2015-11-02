# Makefile for the Docker image gcr.io/$(PROJECT)/expandybird
# MAINTAINER: Jack Greenfield <jackgr@google.com>
# If you update this image please check the tag value before pushing.

.PHONY : all build test push container clean

PREFIX := gcr.io/$(PROJECT)
IMAGE := expandybird
TAG := latest

DIR := .

push: container
	gcloud docker push $(PREFIX)/$(IMAGE):$(TAG)

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
	
