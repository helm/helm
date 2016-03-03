DOCKER_REGISTRY := gcr.io
PREFIX := $(DOCKER_REGISTRY)/$(PROJECT)
IMAGE ?= resourcifier
TAG ?= git-$(shell git rev-parse --short HEAD)
FULL_IMAGE := $(PREFIX)/$(IMAGE)

KUBE_VERSION ?= v1.1.7

.PHONY: container
container: binary kubectl
	docker build -t $(FULL_IMAGE):latest -f Dockerfile .
	docker tag $(FULL_IMAGE):latest $(FULL_IMAGE):$(TAG)

.PHONY: push
push: container
ifeq ($(DOCKER_REGISTRY),gcr.io)
	gcloud docker push $(PREFIX)/$(IMAGE):$(TAG)
else
	docker push $(PREFIX)/$(IMAGE):$(TAG)
endif

binary:
	cp ../../bin/linux-amd64/resourcifier ./bin

kubectl:
	curl -fsSL -o bin/kubectl https://storage.googleapis.com/kubernetes-release/release/${KUBE_VERSION}/bin/linux/amd64/kubectl
	chmod +x bin/kubectl
