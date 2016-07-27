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

Using Helm is as easy as this:

```console
$ helm init                            # Initialize Helm as well as the Tiller server
$ helm install docs/examples/alpine    # Install the example Alpine chart
happy-panda                            # <-- That's the name of your release
$ helm list                            # List all releases
happy-panda
quiet-kitten
```

## Install

Helm is in its early stages of development. At this time there are no binary
releases.

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

## The History of the Project

Kubernetes Helm is the merged result of [Helm
Classic](https://github.com/helm/helm) and the Kubernetes port of GCS Deployment
Manager. The project was jointly started by Google and Deis, though it
is now part of the CNCF.

Differences from Helm Classic:

- Helm now has both a client (`helm`) and a server (`tiller`). The
  server runs inside of Kubernetes, and manages your resources.
- Helm's chart format has changed for the better:
  - Dependencies are immutable and stored inside of a chart's `charts/`
    directory.
  - Charts are strongly versioned using [SemVer 2](http://semver.org/spec/v2.0.0.html)
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
