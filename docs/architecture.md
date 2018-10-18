# The Kubernetes Helm Architecture

This document describes the Helm architecture at a high level.

## The Purpose of Helm

Helm is a tool for managing Kubernetes packages called _charts_. Helm
can do the following:

- Create new charts from scratch
- Package charts into chart archive (tgz) files
- Interact with chart repositories where charts are stored
- Install and uninstall charts into an existing Kubernetes cluster
- Manage the release cycle of charts that have been installed with Helm

For Helm, there are three important concepts:

1. The _chart_ is a bundle of information necessary to create an
   instance of a Kubernetes application.
2. The _config_ contains configuration information that can be merged
   into a packaged chart to create a releasable object.
3. A _release_ is a running instance of a _chart_, combined with a
   specific _config_.

## Components

Helm is an executable which is implemented into two distinct parts:

**The Helm Client** is a command-line client for end users. The client
is responsible for the following:

- Local chart development
- Managing repositories
- Managing releases
- Interfacing with the Helm library
  - Sending charts to be installed
  - Requesting upgrading or uninstalling of existing releases

**The Helm Library** provides the logic for executing all Helm operations.
It interfaces with the Kubernetes API server and provides the following capability:

- Combining a chart and configuration to build a release
- Installing charts into Kubernetes, and providing the subsequent release object
- Upgrading and uninstalling charts by interacting with Kubernetes

The standalone Helm library encapsulates the Helm logic so that it can be leveraged by different clients.

## Implementation

The Helm client and library is written in the Go programming language.

The library uses the Kubernetes client library to communicate with Kubernetes. Currently,
that library uses REST+JSON. It stores information in Secrets located inside of Kubernetes.
It does not need its own database.

Configuration files are, when possible, written in YAML.
