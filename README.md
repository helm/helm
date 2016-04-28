# Kubernetes Helm

Helm is a tool for managing Kubernetes charts. Charts are packages of
pre-configured Kubernetes resources.

## Install

Helm is in its early stages of development. At this time there are no
releases.

To install Helm from source, follow this process:

Make sure you have the prerequisites:
- Go 1.6
- A running Kubernetes cluster
- `kubectl` properly configured to talk to your cluster
- Glide 0.10 or greater

1. Clone (or otherwise download) this repository
2. Run `make boostrap build`

You will now have two binaries built:

- `bin/helm` is the client
- `bin/tiller` is the server

You can locally run Tiller, or you build a Docker image (`make
docker-build`) and then deploy it (`helm init -i IMAGE_NAME`).

The [documentation](docs) folder contains more information about the
architecture and usage of Helm/Tiller.

## The History of the Project

Kubernetes Helm is the merged result of [Helm
Classic](https://github.com/helm/helm) and the Kubernetes port of GCS Deployment
Manager. The project was jointly started by Google and Deis, though it
is now part of the CNCF.
