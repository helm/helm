# Kubernetes Helm

Helm is a tool for managing Kubernetes charts. Charts are packages of
pre-configured Kubernetes resources.

Features:

- Helm now has both a client (`helm`) and a server (`tiller`). The
  server runs inside of Kubernetes, and manages your resources.
- Helm's chart format has changed for the better:
  - Dependencies are immutable and stored inside of a chart's `charts/`
    directory.
  - Charts are strongly versioned using SemVer 2
  - Charts can be loaded from directories or from chart archive files
  - Helm supports Go templates without requiring you to run `generate`
    or `template` commands.
  - Helm makes it easy to configure your releases -- and share the
    configuration with the rest of your team.
- Helm chart repositories now use plain HTTP instead of Git/GitHub.
  There is no longer any GitHub dependency.
  - A chart server is a simple HTTP server
  - Charts are referenced by version
  - The `helm serve` command will run a local chart server, though you
    can easily use object storage (S3, GCS) or a regular web server.
  - And you can still load charts from a local directory.
- The Helm workspace is gone. You can now work anywhere on your
  filesystem that you want to work.

## Install

Helm is in its early stages of development. At this time there are no
releases.

To install Helm from source, follow this process:

Make sure you have the prerequisites:
- Go 1.6
- A running Kubernetes cluster
- `kubectl` properly configured to talk to your cluster
- [Glide](https://glide.sh/) 0.10 or greater with both git and mercurial installed.

1. [Properly set your $GOPATH](https://golang.org/doc/code.html)
2. Clone (or otherwise download) this repository into $GOPATH/src/github.com/kubernetes/helm
3. Run `make bootstrap build`

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
