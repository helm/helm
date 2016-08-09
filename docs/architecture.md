# The Kubernetes Helm Architecture

This document describes the Helm architecture at a high level.

## The Purpose of Helm

Helm is a tool for managing Kubernetes packages called _charts_. Helm
can do the following:

- Create new charts from scratch
- Package charts into chart archive (tgz) files
- Interact with chart repositories where charts are stored
- Install and uninstall charts into an existing Kubernetes cluster
- Manage the releases of charts that have been installed with Helm

For Helm, there are three important concepts:

1. The _chart_ is a bundle of information necessary to create an
   instance of a Kubernetes application.
2. The _config_ contains configuration information that can be merged
   into a packaged chart to create a releasable object.
3. A _release_ is a running instance of a _chart_, combined with a
   specific _config_.

Following the formula made famous by the [12 Factor App](http://12factor.net/), _chart + config
= release_.

## Components

Helm has two major components:

**The Helm Client** is a command-line client for end users. The client
is responsible for the following domains:

- Local chart development
- Managing repositories
- Interacting with the Tiller server
  - Sending charts to be installed
  - Asking for information about releases
  - Requesting upgrading or uninstalling of existing releases

**The Tiller Server** is an in-cluster server that interacts with the
Helm client, and interfaces with the Kubernetes API server. The server
is responsible for the following:

- Listing for incomming requests from the Helm client
- Combining a chart and configuration to build a release
- Installing charts into Kubernetes, and then tracking the subsequent
  release
- Upgrading and uninstalling charts by interacting with Kubernetes

In a nutshell, the client is responsible for managing charts, and the
server is responsible for managing releases.

## Implementation

The Helm client is written in the Go programming language, and uses the
gRPC protocol suite to interact with the Tiller server.

The Tiller server is also written in Go. It provides a gRPC server to
connect with the client, and it uses the Kubernetes client library to
communicate with Kubernetes. Currently, that library uses REST+JSON.

The Tiller server stores information in ConfigMaps located inside of
Kubernetes. It does not need its own database.

### Structure of the Code

The individual programs are located in `cmd/`. Shared libraries are
stored in `pkg/`. The raw ProtoBuf files are stored in `_proto/hapi`
(where `hapi` stands for the Helm Application Programming Interface).
The Go files generated from the `proto` definitions are stored in
`pkg/proto`.

Docker images are built by cross-compiling Linux binaries and then
building a Docker image from the files in `rootfs`.

The `scripts/` directory contains a number of utility scripts. Most of these
are used by the CI/CD pipeline.

Go dependencies are managed with
[Glide](https://github.com/Masterminds/glide) and stored in the
`vendor/` directory.
