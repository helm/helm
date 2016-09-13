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

## Install

Download a [release tarball of helm for your platform](https://github.com/kubernetes/helm/releases). Unpack the `helm` binary and add it to your PATH and you are good to go! OS X/[Cask](https://caskroom.github.io/) users can `brew cask install helm`.

## Docs

- [Quick Start](docs/quickstart.md)
- [Architecture](docs/architecture.md)
- [Charts](docs/charts.md)
	- [Chart Repository Guide](docs/chart_repository.md)
	- [Syncing your Chart Repository](docs/chart_repository_sync_example.md)
- [Developers](docs/developers.md)
