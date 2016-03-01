DOCKER_REGISTRY := gcr.io
PREFIX := $(DOCKER_REGISTRY)/$(PROJECT)
IMAGE ?= expandybird
TAG ?= git-$(shell git rev-parse --short HEAD)
FULL_IMAGE := $(PREFIX)/$(IMAGE)

.PHONY: container
container: binary expansion
	docker build -t $(FULL_IMAGE):latest -f Dockerfile .
	docker tag $(FULL_IMAGE):latest $(FULL_IMAGE):$(TAG)

.PHONY: push
push: container
ifeq ($(DOCKER_REGISTRY),gcr.io)
	gcloud docker push $(PREFIX)/$(IMAGE):$(TAG)
else
	docker push $(PREFIX)/$(IMAGE):$(TAG)
endif

expansion:
	cp -R ../../expansion ./opt

binary:
	cp ../../bin/linux-amd64/expandybird ./bin
