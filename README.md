# Deployment Manager

[![Go Report Card](http://goreportcard.com/badge/kubernetes/deployment-manager)](http://goreportcard.com/report/kubernetes/deployment-manager)

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
[replicated service](https://github.com/kubernetes/application-dm-templates/common/replicatedservice/v1),
which creates a service and replication controller with matching selectors, and
the back end is a 
[Redis cluster](https://github.com/kubernetes/application-dm-templates/storage/redis/v1),
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

Follow these 3 steps to install DM:

1. Make sure your Kubernetes cluster is up and running, and that you can run
`kubectl` commands against it.
1. Clone this repository into the src folder of your GOPATH, if you haven't already.
See the [Kubernetes developer documentation](https://github.com/kubernetes/kubernetes/blob/master/docs/devel/development.md)
for information on how to setup Go and use the repository.
1. Use `kubectl` to install DM into your cluster `kubectl create -f
install.yaml`

That's it. You can now use `kubectl` to see DM running in your cluster:

```
kubectl get pod,rc,service --namespace=dm
```

If you see expandybird-service, manager-service, resourcifier-service, and
expandybird-rc, manager-rc and resourcifier-rc with pods that are READY, then DM
is up and running!

## Using Deployment Manager

### Setting up the client

The easiest way to interact with Deployment Manager is through the `dm` tool
hitting a `kubectl` proxy.

#### Creating a proxy

You can run the following to start a proxy that lets you interact with the
Kubernetes API server through port 8001 on `localhost`

```
kubectl proxy --port=8001 --namespace=dm &
```

`dm` will use
`http://localhost:8001/api/v1/proxy/namespaces/dm/services/manager-service:manager`
as the default service address for the DM service.

#### Getting the client
You can get access to the client in one of two ways:

1. Build the client by running `make` in the deployment-manager repository.
1. Use the client from the container `gcr.io/dm-k8s-testing/dm`.

**NOTE**: If you are using the client from the docker container, you will need
to substitute the following docker command in place of `dm` in all of the
examples below, or alias the command appropriately.

```
alias dm="docker run --net=host gcr.io/dm-k8s-testing/dm"
```

If you are running the docker container on a Mac, you'll also need to substitute
your machine IP to access the Kubernetes proxy running locally:

```
alias dm="docker run gcr.io/dm-k8s-testing/dm --service http://<MACHINE IP ADDRESS>:8001/api/v1/proxy/namespaces/dm/services/manager-service:manager"
```

### Using the client

The DM client, `dm`, can deploy configurations from the command line. It can also
pull templates from a template registry, generate configurations from them using
parameters supplied on the command line, and deploy the resulting configurations.

#### Deploying a configuration

`dm` can deploy a configuration from a file, or read one from `stdin`. This
command deploys the Guestbook example using the configuration shown above from
the examples directory in this project:

```
dm deploy examples/guestbook/guestbook.yaml
```

You can now use `kubectl` to see Guestbook running:

```
kubectl get service
```

Look for frontend-service. If your cluster supports external load balancing, it
will have an external IP assigned to it, and you can navigate to it in your browser
to see the guestbook in action. 

For more information about this example, see [examples/guestbook/README.md](examples/guestbook/README.md)

#### Deploying a template directly

You can also deploy a template directly, without a configuration. This command
deploys a redis cluster with two slaves from the redis template in the [Kubernetes
Template Registry](https://github.com/kubernetes/application-dm-templates):

```
dm deploy storage/redis:v1
```

You can optionally supply values for template parameters on the command line,
like this:

```
dm --properties workers=3 deploy storage/redis:v1
```

When you deploy a template directly, without a configuration, `dm` generates a
configuration from the template and the supplied parameters, and then deploys the
configuration.

For more information about deploying templates from a template registry or adding
types to a template registry, see [the template registry documentation](docs/templates/registry.md).

### Additional commands

Here's a list of available `dm` commands:

```
expand              Expands the supplied configuration(s)
deploy              Deploys the named template or the supplied configuration(s)
list                Lists the deployments in the cluster
get                 Retrieves the named deployment
delete              Deletes the named deployment
update              Updates a deployment using the supplied configuration(s)
deployed-types      Lists the types deployed in the cluster
deployed-instances  Lists the instances of the named type deployed in the cluster
templates           Lists the templates in a given template registry
describe            Describes the named template in a given template registry
```

## Uninstalling Deployment Manager

You can uninstall Deployment Manager using the same configuration:

```
kubectl delete -f install.yaml
```

## Building the Container Images

This project runs Deployment Manager on Kubernetes as three replicated services.
By default, install.yaml uses prebuilt images stored in Google Container Registry
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

