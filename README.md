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

Binary downloads of the Beta.2 Helm client can be found at the following links:

- [OSX](http://storage.googleapis.com/kubernetes-helm/helm-v2.0.0-beta.2-darwin-amd64.tar.gz)
- [Linux](http://storage.googleapis.com/kubernetes-helm/helm-v2.0.0-beta.2-linux-amd64.tar.gz)
- [Linux 32-bit](http://storage.googleapis.com/kubernetes-helm/helm-v2.0.0-beta.2-linux-386.tar.gz)

Unpack the `helm` binary and add it to your PATH and you are good to go! OS X/[Cask](https://caskroom.github.io/) users can `brew cask install helm`.

To rapidly get Helm up and running, start with the [Quick Start Guide](docs/quickstart.md).

See the [installation guide](docs/install.md) for more options,
including installing pre-releases.


## Docs

- [Quick Start](docs/quickstart.md)
- [Installing Helm](docs/install.md)
- [Using Helm](docs/using_helm.md)
- [Developing Charts](docs/charts.md)
	- [Chart Lifecycle Hooks](docs/charts_hooks.md)
	- [Chart Tips and Tricks](docs/charts_tips_and_tricks.md)
	- [Chart Repository Guide](docs/chart_repository.md)
	- [Syncing your Chart Repository](docs/chart_repository_sync_example.md)
	- [Signing Charts](docs/provenance.md)
- [Architecture](docs/architecture.md)
- [Developers](docs/developers.md)
- [History](docs/history.md)
- [Glossary](docs/glossary.md)

## Community, discussion, contribution, and support

You can reach the Helm community and developers via the following channels:

- [Kubernetes Slack](https://slack.k8s.io): #helm
- Mailing List: https://groups.google.com/forum/#!forum/kubernetes-sig-apps
- Developer Call: Thursdays at 9:30-10:00 Pacific. https://engineyard.zoom.us/j/366425549

### Code of conduct

Participation in the Kubernetes community is governed by the [Kubernetes Code of Conduct](code-of-conduct.md).
