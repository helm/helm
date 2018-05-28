# Guestbook

[Guestbook](https://github.com/kubernetes/examples/tree/master/guestbook) is a simple, multi-tier PHP-based web application that uses redis chart.
## TL;DR;

```console
$ helm install stable/guestbook
```

## Introduction

This chart bootstraps a [guestbook](https://github.com/kubernetes/examples/tree/master/guestbook) deployment on a [Kubernetes](http://kubernetes.io) cluster using the [Helm](https://helm.sh) package manager.

It also packages the [Bitnami Redis chart](https://github.com/kubernetes/charts/tree/master/stable/redis) which is required for bootstrapping a Redis deployment for the caching requirements of the guestbook application.

## Prerequisites

- Kubernetes 1.4+ with Beta APIs enabled
- PV provisioner support in the underlying infrastructure

## Installing the Chart

To install the chart with the release name `my-release`:

```console
$ helm install --name my-release stable/guestbook
```

The command deploys the guestbook on the Kubernetes cluster in the default configuration. The [configuration](#configuration) section lists the parameters that can be configured during installation.

> **Tip**: List all releases using `helm list`

## Uninstalling the Chart

To uninstall/delete the `my-release` deployment:

```console
$ helm delete my-release
```

The command removes all the Kubernetes components associated with the chart and deletes the release.

## Configuration

The following tables lists the configurable parameters of the WordPress chart and their default values.

| Parameter                            | Description                                | Default                                                    |
| -------------------------------      | -------------------------------            | ---------------------------------------------------------- |
| `image`                              | apapche-php image                          | `google-samples/gb-frontend:{VERSION}`                     |
| `imagePullPolicy`                    | Image pull policy                          | `IfNotPresent`                                             |
| `nodeSelector`                       | Node labels for pod assignment             | `{}`                                                       |

The above parameters map to the env variables defined in [bitnami/wordpress](http://github.com/bitnami/bitnami-docker-wordpress). For more information please refer to the [bitnami/wordpress](http://github.com/bitnami/bitnami-docker-wordpress) image documentation.

Specify each parameter using the `--set key=value[,key=value]` argument to `helm install`. For example,

```console
$ helm install --name my-release \
  --set redis.usePassword=false \
    stable/guestbook
```

```console
$ helm install --name my-release -f values.yaml stable/guestbook
```

> **Tip**: You can use the default [values.yaml](values.yaml)

