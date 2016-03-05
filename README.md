# Helm

[![Circle CI](https://circleci.com/gh/kubernetes/helm.svg?style=svg)](https://circleci.com/gh/kubernetes/helm) [![Go Report Card](http://goreportcard.com/badge/kubernetes/helm)](http://goreportcard.com/report/kubernetes/helm)

Helm makes it easy to create, describe, update and
delete Kubernetes resources using declarative configuration. A configuration is
just a `YAML` file that configures Kubernetes resources or supplies parameters
to templates.

Helm Manager runs server side, in your Kubernetes cluster, so it can tell you what templates
you've instantiated there, what resources they created, and even how the resources
are organized. So, for example, you can ask questions like:

* What Redis instances are running in this cluster?
* What Redis master and slave services are part of this Redis instance?
* What pods are part of this Redis slave?

Please hang out with us in [the Slack chat room](https://kubernetes.slack.com/messages/helm/).

## Installing Helm

Note: if you're exploring or using the project, you'll probably want to pull
(the latest release)[https://github.com/kubernetes/helm/releases/latest],
since there may be undiscovered or unresolved issues at HEAD.

From a Linux or Mac OS X client:

```
$ git clone https://github.com/kubernetes/deployment-manager.git
$ cd deployment-manager
$ make build
$ bin/helm dm install
```

That's it. You can now use `kubectl` to see DM running in your cluster:

```
kubectl get pod,rc,service --namespace=dm
```

If you see expandybird-service, manager-service, resourcifier-service, and
expandybird-rc, manager-rc and resourcifier-rc with pods that are READY, then DM
is up and running!

## Using Helm

Run a Kubernetes proxy to allow the dm client to connect to the cluster:

```
kubectl proxy --port=8001 --namespace=dm &
```

## Uninstalling Helm from Kubernetes

You can uninstall Deployment Manager using the same configuration:

```
helm dm uninstall
```

## Design of Deployment Manager

There is a more detailed [design document](docs/design/design.md) available.

## Status of the Project

This project is still under active development, so you might run into issues. If
you do, please don't be shy about letting us know, or better yet, contribute a
fix or feature.

## Contributing
Your contributions are welcome.

We use the same [workflow](https://github.com/kubernetes/kubernetes/blob/master/docs/devel/development.md#git-setup),
[License](LICENSE) and [Contributor License Agreement](CONTRIBUTING.md) as the main Kubernetes repository.

## Relationship to Google Cloud Platform
DM uses many of the same concepts and languages as
[Google Cloud Deployment Manager](https://cloud.google.com/deployment-manager/overview),
but creates resources in Kubernetes clusters, not in Google Cloud Platform projects.
