# Quickstart Guide

This guide covers how you can quickly get started using Helm.

## Prerequisites

- You must have Kubernetes installed, and have a local configured copy
  of `kubectl`.

## Install Helm

Download a binary release of the Helm client from the official project
page.

Alternately, you can clone the GitHub project and build your own
client from source. The quickest route to installing from source is to
run `make bootstrap build`, and then use `bin/helm`.

## Initialize Helm and Install Tiller

Once you have Helm ready, you can initialize the local CLI and also
install Tiller into your Kubernetes cluster in one step:

```console
$ helm init
```

## Install an Existing Chart

To install an existing chart, you can run the `helm install` command:

_TODO:_ Might need instructions about repos.

```console
$ helm install nginx-1.0.0
Released smiling-penguin
```

In the example above, the `nginx` chart was released, and the name of
our new release is `smiling-penguin`

## Learn About The Release

To find out about our release, run `helm status`:

```console
$ helm status smiling-penguin
Status: DEPLOYED
```

## Uninstall a Release

To uninstall a release, use the `helm delete` command:

```console
$ helm delete smiling-penguin
Removed smiling-penguin
```

This will uninstall `smiling-penguin` from Kubernetes, but you will
still be able to request information about that release:

```console
$ helm status smiling-penguin
Status: DELETED
```

## Reading the Help Text

To learn more about the available Helm commands, use `helm help` or type
a command followed by the `-h` flag:

```console
$ helm get -h
```
