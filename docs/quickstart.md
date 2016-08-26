# Quickstart Guide

This guide covers how you can quickly get started using Helm.

## Prerequisites

- You must have Kubernetes installed, and have a local configured copy
  of `kubectl`.

## Install Helm

Download a binary release of the Helm client from 
[the official project page](https://github.com/kubernetes/helm/releases).

Alternately, you can clone the GitHub project and build your own
client from source. The quickest route to installing from source is to
run `make bootstrap build`, and then use `bin/helm`.

## Initialize Helm and Install Tiller

Once you have Helm ready, you can initialize the local CLI and also
install Tiller into your Kubernetes cluster in one step:

```console
$ helm init
```

## Install an Example Chart

To install a chart, you can run the `helm install` command. 
Let's use an example chart from this repository. 
Make sure you are in the root directory of this repo.


```console
$ helm install docs/examples/alpine
Released smiling-penguin
```

In the example above, the `alpine` chart was released, and the name of
our new release is `smiling-penguin`. You can view the details of the chart we just 
installed by taking a look at the nginx chart in 
[docs/examples/alpine/Chart.yaml](examples/alpine/Chart.yaml).

## Change a Default Chart Value

A nice feature of helm is the ability to change certain values of the package for the install.
Let's install the `nginx` example from this repository but change the `replicaCount` to 7.

```console
$ helm install --set replicaCount=7 docs/examples/nginx
happy-panda
```

You can view the chart for this example in 
[docs/examples/nginx/Chart.yaml](examples/nginx/Chart.yaml) and the default values in
[docs/examples/nginx/values.yaml](examples/nginx/values.yaml).

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
