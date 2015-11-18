.PHONY: all build push clean

DOCKER_REGISTRY = gcr.io
PREFIX = $(DOCKER_REGISTRY)/$(PROJECT)
IMAGE = storm-base
TAG = latest

DIR = .

all: build

build:
» docker build -t $(PREFIX)/$(IMAGE):$(TAG) $(DIR)

push: build
ifeq ($(DOCKER_REGISTRY),gcr.io)
» gcloud docker push $(PREFIX)/$(IMAGE):$(TAG)
else
» docker push $(PREFIX)/$(IMAGE):$(TAG)
endif

clean:
» docker rmi $(PREFIX)/$(IMAGE):$(TAG)
