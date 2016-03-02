#Under Construction

#Getting Started Guide

[Helm](https://helm.sh) helps you find and use software built for Kubernetes. With a few Helm commands you can quickly and easily deploy software packages like:

- Postgres
- etcd
- HAProxy
- redis

All of the Helm charts live at [github.com/kubernetes/charts](https://github.com/kubernetes/charts). If you want to make your own charts we have a guide for [authoring charts](authoring_charts.md). Charts should follow the defined [chart format](/docs/design/chart_format.md).

Get started with the following steps:

1. [Clone](github.com/kubernetes/helm) this project.

2. Then, run `make build`. This will install all the binaries you need in `bin/` in the root of the directory.

3. To see a list of helm commands, run `./bin/helm`

4. `helm dm install` to install the server side component.
