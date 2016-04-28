# Developers Guide

This guide explains how to set up your environment for developing on
Helm and Tiller.

## Prerequisites

- Go 1.6.0 or later
- Glide 0.10.2 or later
- kubectl 1.2 or later
- A Kubernetes cluster (optional)
- The gRPC toolchain

## Building Helm/Tiller

We use Make to build our programs. The simplest way to get started is:

```console
$ make boostrap build
```

This will build both Helm and Tiller.

To run all of the tests (without running the tests for `vendor/`), run
`make test`.

To run Helm and Tiller locally, you can run `bin/helm` or `bin/tiller`.

- Helm and Tiller are known to run on Mac OSX and most Linuxes, including
  Alpine.
- Tiller must have access to a Kubernets cluster. It learns about the
  cluster by examining the Kube config files that `kubectl` uese.

## gRPC and Protobuf

Helm and Tiller communicate using gRPC. To get started with gRPC, you will need to...

- Install `protoc` for compiling protobuf files. Releases are
  [here](https://github.com/google/protobuf/releases)
- Install the protoc Go plugin: `go get -u github.com/golang/protobuf/protoc-gen-go`

Note that you need to be on protobuf 3.x (`protoc --version`) and use the latest Go plugin.

### The Helm API (HAPI)

We use gRPC as an API layer. See `pkg/proto/hapi` for the generated Go code,
and `_proto` for the protocol buffer definitions.

To regenerate the Go files from the protobuf source, `cd _proto &&
make`.

## Docker Images

To build Docker images, use `make docker-build`

## Running a Local Cluster

You can run tests locally using the `scripts/local-cluster.sh` script to
start Kubernetes inside of a Docker container. For OS X, you will need
to be running `docker-machine`.

## Contribution Guidelines

We welcome contributions. This project has set up some guidelines in
order to ensure that (a) code quality remains high, (b) the project
remains consistent, and (c) contributions follow the open source legal
requirements. Our intent is not to burden contributors, but to build
elegant and high-quality open source code so that our users will benefit.

Make sure you have read and understood the main CONTRIBUTING guide:

https://github.com/kubernetes/helm/blob/master/CONTRIBUTING.md

We follow the coding standards and guidelines outlined by the Deis
project:

https://github.com/deis/workflow/blob/master/CONTRIBUTING.md
https://github.com/deis/workflow/blob/master/src/contributing/submitting-a-pull-request.md

Adidtionally, contributors must have a CLA with CNCF/Google before we can
accept contributions.
