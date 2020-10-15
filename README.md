# Helm

[![CircleCI](https://circleci.com/gh/helm/helm.svg?style=shield)](https://circleci.com/gh/helm/helm)
[![Go Report Card](https://goreportcard.com/badge/github.com/helm/helm)](https://goreportcard.com/report/github.com/helm/helm)
[![GoDoc](https://img.shields.io/static/v1?label=godoc&message=reference&color=blue)](https://pkg.go.dev/helm.sh/helm/v3)
[![CII Best Practices](https://bestpractices.coreinfrastructure.org/projects/3131/badge)](https://bestpractices.coreinfrastructure.org/projects/3131)

Helm is a tool for managing Charts. Charts are packages of pre-configured Kubernetes resources.

Use Helm to:

- Find and use [popular software packaged as Helm Charts](https://hub.helm.sh) to run in Kubernetes
- Share your own applications as Helm Charts
- Create reproducible builds of your Kubernetes applications
- Intelligently manage your Kubernetes manifest files
- Manage releases of Helm packages

## Helm in a Handbasket

Helm is a tool that streamlines installing and managing Kubernetes applications.
Think of it like apt/yum/homebrew for Kubernetes.

- Helm renders your templates and communicates with the Kubernetes API
- Helm runs on your laptop, CI/CD, or wherever you want it to run.
- Charts are Helm packages that contain at least two things:
  - A description of the package (`Chart.yaml`)
  - One or more templates, which contain Kubernetes manifest files
- Charts can be stored on disk, or fetched from remote chart repositories
  (like Debian or RedHat packages)

## Install


Binary downloads of the Helm client can be found on [the Releases page](https://github.com/helm/helm/releases/latest).

Unpack the `helm` binary and add it to your PATH and you are good to go!

If you want to use a package manager:

- [Homebrew](https://brew.sh/) users can use `brew install helm`.
- [Chocolatey](https://chocolatey.org/) users can use `choco install kubernetes-helm`.
- [Scoop](https://scoop.sh/) users can use `scoop install helm`.
- [GoFish](https://gofi.sh/) users can use `gofish install helm`.
- [Snapcraft](https://snapcraft.io/) users can use `snap install helm --classic`

To rapidly get Helm up and running, start with the [Quick Start Guide](https://helm.sh/docs/intro/quickstart/).

See the [installation guide](https://helm.sh/docs/intro/install/) for more options,
including installing pre-releases.

## Docs

Get started with the [Quick Start guide](https://helm.sh/docs/intro/quickstart/) or plunge into the [complete documentation](https://helm.sh/docs)

## Roadmap

The [Helm roadmap uses Github milestones](https://github.com/helm/helm/milestones) to track the progress of the project.

## Community, discussion, contribution, and support

You can reach the Helm community and developers via the following channels:

- [Kubernetes Slack](https://kubernetes.slack.com):
  - [#helm-users](https://kubernetes.slack.com/messages/helm-users)
  - [#helm-dev](https://kubernetes.slack.com/messages/helm-dev)
  - [#charts](https://kubernetes.slack.com/messages/charts)
- Mailing List:
  - [Helm Mailing List](https://lists.cncf.io/g/cncf-helm)
- Developer Call: Thursdays at 9:30-10:00 Pacific ([meeting details](https://github.com/helm/community/blob/master/communication.md#meetings))

### Code of conduct

Participation in the Helm community is governed by the [Code of Conduct](code-of-conduct.md).
