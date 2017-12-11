# Quickstart Guide

This guide covers how you can quickly get started using Helm.

## Prerequisites

The following prerequisites are required for a successful and properly secured use of Helm.

1. A Kubernetes cluster
2. Deciding what security configurations to apply to your installation, if any
3. Installing and configuring Helm and Tiller, the cluster-side service.


### Install Kubernetes or have access to a cluster
- You must have Kubernetes installed. For the latest release of Helm, we recommend the latest stable release of Kubernetes, which in most cases is the second-latest minor release. 
- You should also have a local configured copy of `kubectl`.

NOTE: Kubernetes versions prior to 1.6 have limited or no support for role-based access controls (RBAC).

Helm will figure out where to install Tiller by reading your Kubernetes
configuration file (usually `$HOME/.kube/config`). This is the same file
that `kubectl` uses.

To find out which cluster Tiller would install to, you can run
`kubectl config current-context` or `kubectl cluster-info`.

```console
$ kubectl config current-context
my-cluster
```

### Understand your Security Context

As with all powerful tools, ensure you are installing it correctly for your scenario.

If you're using Helm on a cluster that you completely control, like minikube or a cluster on a private network in which sharing is not a concern, the default installation -- which applies no security configuration -- is fine, and it's definitely the easiest. To install Helm without additional security steps, [install Helm](#Install-Helm) and then [initialize Helm](#initialize-helm-and-install-tiller).

However, if your cluster is exposed to a larger network or if you share your cluster with others -- production clusters fall into this category -- you must take extra steps to secure your installation to prevent careless or malicious actors from damaging the cluster or its data. To apply configurations that secure Helm for use in production environments and other multi-tenant scenarios, see [Securing a Helm installation](securing_installation.md)

If your cluster has Role-Based Access Control (RBAC) enabled, you may want
to [configure a service account and rules](rbac.md) before proceeding.

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

By default, when Tiller is installed,it does not have authentication enabled.
To learn more about configuring strong TLS authentication for Tiller, consult
[the Tiller TLS guide](tiller_ssl.md).

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
NAME             VERSION   UPDATED                   STATUS    CHART
smiling-penguin  1         Wed Sep 28 12:59:46 2016  DEPLOYED  mysql-0.1.0
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
