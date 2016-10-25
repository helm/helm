# Helm Glossary

Helm uses a few special terms to describe components of the
architecture.

## Chart

A Helm package that contains information sufficient for installing a set
of Kubernetes resources into a Kubernetes cluster.

Charts contain a `Chart.yaml` file as well as templates, default values
(`values.yaml`), and dependencies.

Charts are developed in a well-defined directory structure, and then
packaged into an archive format called a _chart archive_.

## Chart Archive

A _chart archive_ is a tarred and gzipped (and optionally signed) chart.

## Chart Dependency (Subcharts)

Charts may depend upon other charts. There are two ways a dependency may
occur:

- Soft dependency: A chart may simply not function without another chart
  being installed in a cluster. Helm does not provide tooling for this
  case. In this case, dependencies may be managed separately.
- Hard dependency: A chart may contain (inside of its `charts/`
  directory) another chart upon which it depends. In this case,
  installing the chart will install all of its dependencies. In this
  case, a chart and its dependencies are managed as a collection.

When a chart is packaged (via `helm package`) all of its hard dependencies
are bundled with it.

## Chart Version

Charts are versioned according to the [SemVer 2
spec](http://semver.org). A version number is required on every chart.

## Chart.yaml

Information about a chart is stored in a special file called
`Chart.yaml`. Every chart must have this file.

## Helm (and helm)

Helm is the package manager for Kubernetes. As an operating system
package manager makes it easy to install tools on an OS, Helm makes it
easy to install applications and resources into Kubernetes clusters.

While _Helm_ is the name of the project, the command line client is also
named `helm`. By convention, when speaking of the project, _Helm_ is
capitalized. When speaking of the client, _helm_ is in lowercase.

## Helm Home (HELM_HOME)

The Helm client stores information in a local directory referred to as
_helm home_. By default, this is in the `$HOME/.helm` directory.

This directory contains configuration and cache data, and is created by
`helm init`.

## Kube Config (KUBECONFIG)

The Helm client learns about Kubernetes clusters by using files in the _Kube
config_ file format. By default, Helm attempts to find this file in the
place where `kubectl` creates it (`$HOME/.kube/config`).

## Lint (Linting)

To _lint_ a chart is to validate that it follows the conventions and
requirements of the Helm chart standard. Helm provides tools to do this,
notably the `helm lint` command.

## Provenance (Provenance file)

Helm charts may be accompanied by a _provenance file_ which provides
information about where the chart came from and what it contains.

Provenance files are one part of the Helm security story. A provenance contains
a cryptographic hash of the chart archive file, the Chart.yaml data, and
a signature block (an OpenPGP "clearsign" block). When coupled with a
keychain, this provides chart users with the ability to:

- Validate that a chart was signed by a trusted party
- Validate that the chart file has not been tampered with
- Validate the contents of a chart metadata (`Chart.yaml`)
- Quickly match a chart to its provenance data

Provenance files have the `.prov` extension, and can be served from a
chart repository server or any other HTTP server.

## Release

When a chart is installed, Tiller (the Helm server) creates a _release_
to track that installation.

A single chart may be installed many times into the same cluster, and
create many different releases. For example, one can install three
PostgreSQL databases by running `helm install` three times with a
different release name.

(Prior to 2.0.0-Alpha.1, releases were called _deployments_. But this
caused confusion with the Kubernetes _Deployment_ kind.)

## Release Number (Release Version)

A single release can be updated multiple times. A sequential counter is
used to track releases as they change. After a first `helm install`, a
release will have _release number_ 1. Each time a release is upgraded or
rolled back, the release number will be incremented.

## Rollback

A release can be upgraded to a newer chart or configuration. But since
release history is stored, a release can also be _rolled back_ to a
previous release number. This is done with the `helm rollback` command.

Importantly, a rolled back release will receive a new release number.

Operation | Release Number
----------|---------------
install   | release 1
upgrade   | release 2
upgrade   | release 3
rollback 1| release 4 (but running the same config as release 1)

The above table illustrates how release numbers increment across
install, upgrade, and rollback.

## Tiller

Tiller is the in-cluster component of Helm. It interacts directly with
the Kubernetes API server to install, upgrade, query, and remove
Kubernetes resources. It also stores the objects that represent
releases.

## Repository (Repo, Chart Repository)

Helm charts may be stored on dedicated HTTP servers called _chart
repositories_ (_repositories_, or just _repos_).

A chart repository server is a simple HTTP server that can serve an
`index.yaml` file that describes a batch of charts, and provides
information on where each chart can be downloaded from. (Many chart
repositories serve the charts as well as the `index.yaml` file.)

A Helm client can point to zero or more chart repositories. By default,
Helm clients point to the `stable` official Kubernetes chart
repository.

## Values (Values Files, values.yaml)

Values provide a way to override template defaults with your own
information.

Helm Charts are "parameterized", which means the chart developer may
expose configuration that can be overridden at installation time. For
example, a chart may expose a `username` field that allows setting a
user name for a service.

These exposed variables are called _values_ in Helm parlance.

Values can be set during `helm install` and `helm upgrade` operations,
either by passing them in directly, or by uploading a `values.yaml`
file.


