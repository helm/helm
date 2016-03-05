# Deployment Manager

[![Build Status](https://travis-ci.org/kubernetes/deployment-manager.svg?branch=master)](https://travis-ci.org/kubernetes/deployment-manager) [![Go Report Card](http://goreportcard.com/badge/kubernetes/deployment-manager)](http://goreportcard.com/report/kubernetes/deployment-manager)

Deployment Manager (DM) `dm` makes it easy to create, describe, update and
delete Kubernetes resources using declarative configuration. A configuration is
just a `YAML` file that configures Kubernetes resources or supplies parameters
to templates. Templates are just YAML files with [Jinja](http://jinja.pocoo.org/)
mark up or Python scripts.

For example, this simple configuration deploys the Guestbook example:

```
resources:
- name: frontend
  type: github.com/kubernetes/application-dm-templates/common/replicatedservice:v1
  properties:
    service_port: 80
    container_port: 80
    external_service: true
    replicas: 3
    image: gcr.io/google_containers/example-guestbook-php-redis:v3
- name: redis
  type: github.com/kubernetes/application-dm-templates/storage/redis:v1
  properties: null
```

It uses two templates. The front end is a 
[replicated service](https://github.com/kubernetes/application-dm-templates/tree/master/common/replicatedservice/v1),
which creates a service and replication controller with matching selectors, and
the back end is a 
[Redis cluster](https://github.com/kubernetes/application-dm-templates/tree/master/storage/redis/v1),
which creates a Redis master and two Redis slaves.

Templates can use other templates, making it easy to create larger structures
from smaller building blocks. For example, the Redis template uses the replicated
service template to create the Redis master, and then again to create each Redis
slave.

DM runs server side, in your Kubernetes cluster, so it can tell you what templates
you've instantiated there, what resources they created, and even how the resources
are organized. So, for example, you can ask questions like:

* What Redis instances are running in this cluster?
* What Redis master and slave services are part of this Redis instance?
* What pods are part of this Redis slave?

Because DM stores its state in the cluster, not on your workstation, you can ask
those questions from any client at any time.

Templates live in ordinary Github repositories called template registries. See
the [Kubernetes Template Registry](https://github.com/kubernetes/application-dm-templates)
for curated Kubernetes applications using Deployment Manager templates.

For more information about configurations and templates, see the
[design document](docs/design/design.md#types).

Please hang out with us in
[the Slack chat room](https://kubernetes.slack.com/messages/sig-configuration/)
and/or [the Google Group](https://groups.google.com/forum/#!forum/kubernetes-sig-config)
for the Kubernetes configuration SIG.

## Installing Deployment Manager

From a Linux or Mac OS X client:

```
curl -s https://raw.githubusercontent.com/kubernetes/deployment-manager/master/get-install.sh | sh
```

and then install the DM services into your Kubernetes cluster:

```
helm dm install
```

That's it. You can now use `kubectl` to see DM running in your cluster:

```
kubectl get pod,rc,service --namespace=dm
```

If you see expandybird-service, manager-service, resourcifier-service, and
expandybird-rc, manager-rc and resourcifier-rc with pods that are READY, then DM
is up and running!

## Using Deployment Manager

Run a Kubernetes proxy to allow the dm client to connect to the cluster:

```
kubectl proxy --port=8001 --namespace=dm &
```

### Deploy an app
To deploy a simple guestbook app:

```
$ dm deploy examples/guestbook/guestbook.yaml
$ kubectl get service
```

The `frontend-service` should have an external IP that you can navigate to in
your browser to play with.

For more information about this example, see [examples/guestbook/README.md](examples/guestbook/README.md)

### Deploying a template

To deploy a redis template from the [Kubernetes
Template Registry](https://github.com/kubernetes/application-dm-templates):

```
dm --properties workers=3 deploy storage/redis:v1
```

For more information about deploying templates from a template registry or adding
types to a template registry, see
[the template registry documentation](docs/templates/registry.md).

## Uninstalling Deployment Manager

You can uninstall Deployment Manager using the same configuration:

```
helm dm delete
```

## Building the Container Images

This project runs Deployment Manager on Kubernetes as three replicated services.
By default, `helm` uses prebuilt images stored in Google Container Registry
to install them. However, you can build your own container images and push them
to your own project in the Google Container Registry: 

1. Set the environment variable `PROJECT` to the name of a project known to
GCloud.
1. Run `make push`

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
DM uses the same concepts and languages as
[Google Cloud Deployment Manager](https://cloud.google.com/deployment-manager/overview),
but creates resources in Kubernetes clusters, not in Google Cloud Platform projects.

