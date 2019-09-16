# Build Guide

This guide explains how to set up your environment for developing on
Helm and Tiller.

## Prerequisites

- The latest version of Go
- The latest version of Glide
- Git

## Building Helm/Tiller

We use Make to build our programs. The simplest way to get started is:

```console
$ cd $GOPATH
$ mkdir -p src/helm.sh
$ cd src/helm.sh
$ git clone https://github.com/helm/helm.git
$ cd helm
$ make build
```

NOTE: If not running from the path `$GOPATH/src/helm.sh/helm`, build would fail or `helm` binary is not work with own code. The
directory `k8s.io` should not be a symlink or `build` will not find the relevant
packages.
