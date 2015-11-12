.PHONY: all build push clean

PREFIX = gcr.io/$(PROJECT)
IMAGE = nginx
TAG = latest

DIR = .

all: build

build:
	docker build -t $(PREFIX)/$(IMAGE):$(TAG) $(DIR)

push: build
	gcloud docker push $(PREFIX)/$(IMAGE):$(TAG)

clean:
	docker rmi $(PREFIX)/$(IMAGE):$(TAG)
