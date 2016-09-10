# Kubernetes Helm

[![CircleCI](https://circleci.com/gh/kubernetes/helm.svg?style=svg)](https://circleci.com/gh/kubernetes/helm)

Helm is a tool for managing Kubernetes charts. Charts are packages of
pre-configured Kubernetes resources.

Use Helm to...

- Find and use popular software packaged as Kubernetes charts
- Share your own applications as Kubernetes charts
- Create reproducible builds of your Kubernetes applications
- Intelligently manage your Kubernetes manifest files
- Manage releases of Helm packages

## Helm in a Handbasket

Helm is a tool that streamlines installing and managing Kubernetes applications.
Think of it like apt/yum/homebrew for Kubernetes.

- Helm has two parts: a client (`helm`) and a server (`tiller`)
- Tiller runs inside of your Kubernetes cluster, and manages releases (installations)
  of your charts.
- Helm runs on your laptop, CI/CD, or wherever you want it to run.
- Charts are Helm packages that contain at least two things:
  - A description of the package (`Chart.yaml`)
  - One or more templates, which contain Kubernetes manifest files
- Charts can be stored on disk, or fetched from remote chart repositories
  (like Debian or RedHat packages)

## Docs

- [Quick Start](docs/quickstart.md)
- [Architechture](docs/architecture.md)
- [Charts](docs/charts.md)
	- [Chart Repository Guide](docs/chart_repository.md)
	- [Syncing your Chart Repository](docs/chart_repository_sync_example.md)
- [Developers](docs/developers.md)


## Install

Download a [release tarball of helm and tiller for your platform](https://github.com/kubernetes/helm/releases). Unpack the `helm` and `tiller` binaries and add them to your PATH and you are good to go! OS X/[Cask](https://caskroom.github.io/) users can `brew cask install helm`.

### Install from source

To install Helm from source, follow this process:

Make sure you have the prerequisites:
- Go 1.6
- A running Kubernetes cluster
- `kubectl` properly configured to talk to your cluster
- [Glide](https://glide.sh/) 0.10 or greater with both git and mercurial installed.

1. [Properly set your $GOPATH](https://golang.org/doc/code.html)
2. Clone (or otherwise download) this repository into $GOPATH/src/k8s.io/helm
3. Run `make bootstrap build`

You will now have two binaries built:

- `bin/helm` is the client
- `bin/tiller` is the server

From here, you can run `bin/helm` and use it to install a recent snapshot of
Tiller. Helm will use your `kubectl` config to learn about your cluster.

For development on Tiller, you can locally run Tiller, or you build a Docker
image (`make docker-build`) and then deploy it (`helm init -i IMAGE_NAME`).

The [documentation](docs) folder contains more information about the
architecture and usage of Helm/Tiller.
