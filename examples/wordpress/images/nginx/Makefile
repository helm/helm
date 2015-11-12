.PHONY: all build push clean

<<<<<<< HEAD
<<<<<<< HEAD
DOCKER_REGISTRY = gcr.io
PREFIX = $(DOCKER_REGISTRY)/$(PROJECT)
=======
PREFIX = gcr.io/$(PROJECT)
>>>>>>> 320a0e9... Adds Wordpress example
=======
DOCKER_REGISTRY = gcr.io
PREFIX = $(DOCKER_REGISTRY)/$(PROJECT)
>>>>>>> a893ea4... Updates Makefile to use generic Docker registry
IMAGE = nginx
TAG = latest

DIR = .

all: build

build:
	docker build -t $(PREFIX)/$(IMAGE):$(TAG) $(DIR)

push: build
<<<<<<< HEAD
<<<<<<< HEAD
=======
>>>>>>> a893ea4... Updates Makefile to use generic Docker registry
ifeq ($(DOCKER_REGISTRY),gcr.io)
	gcloud docker push $(PREFIX)/$(IMAGE):$(TAG)
else
	docker push $(PREFIX)/$(IMAGE):$(TAG)
endif
<<<<<<< HEAD
=======
	gcloud docker push $(PREFIX)/$(IMAGE):$(TAG)
>>>>>>> 320a0e9... Adds Wordpress example
=======
>>>>>>> a893ea4... Updates Makefile to use generic Docker registry

clean:
	docker rmi $(PREFIX)/$(IMAGE):$(TAG)
