# Quickstart Guide

This guide covers how you can quickly get started using Helm.

## Prerequisites

- You must have Kubernetes installed. We recommend version 1.4.1 or
  later.
- You should also have a local configured copy of `kubectl`.

Helm will figure out where to install Tiller by reading your Kubernetes
configuration file (usually `$HOME/.kube/config`). This is the same file
that `kubectl` uses.

To find out which cluster Tiller would install to, you can run
`kubectl config current-context` or `kubectl cluster-info`.

```console
$ kubectl config current-context
my-cluster
```

## Install Helm

Download a binary release of the Helm client. You can use tools like
`homebrew`, or look at [the official releases page](https://github.com/kubernetes/helm/releases).

For more details, or for other options, see [the installation
guide](install.md).

## Initialize Helm and Install Tiller

Once you have Helm ready, you can initialize the local CLI and also
install Tiller into your Kubernetes cluster in one step:

```console
$ helm init
```

This will install Tiller into the Kubernetes cluster you saw with
`kubectl config current-context`.

**TIP:** Want to install into a different cluster? Use the
`--kube-context` flag.

**TIP:** When you want to upgrade Tiller, just run `helm init --upgrade`.

## Install an Example Chart

To install a chart, you can run the `helm install` command. Helm has
several ways to find and install a chart, but the easiest is to use one
of the official `stable` charts.

```console
$ helm repo update              # Make sure we get the latest list of charts
$ helm install stable/mysql
Released smiling-penguin
```

In the example above, the `stable/mysql` chart was released, and the name of
our new release is `smiling-penguin`. You get a simple idea of the
features of this MySQL chart by running `helm inspect stable/mysql`.

Whenever you install a chart, a new release is created. So one chart can
be installed multiple times into the same cluster. And each can be
independently managed and upgraded.

The `helm install` command is a very powerful command with many
capabilities. To learn more about it, check out the [Using Helm
Guide](using_helm.md)

## Learn About Releases

It's easy to see what has been released using Helm:

```console
$ helm ls
NAME           	VERSION	 UPDATED                       	STATUS         	CHART
smiling-penguin	 1      	Wed Sep 28 12:59:46 2016      	DEPLOYED       	mysql-0.1.0
```

The `helm list` function will show you a list of all deployed releases.

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
...
```

Because Helm tracks your releases even after you've deleted them, you
can audit a cluster's history, and even undelete a release (with `helm
rollback`).

## Reading the Help Text

To learn more about the available Helm commands, use `helm help` or type
a command followed by the `-h` flag:

```console
$ helm get -h
```
