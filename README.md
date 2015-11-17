# Deployment Manager

[![Go Report Card](http://goreportcard.com/badge/kubernetes/deployment-manager)](http://goreportcard.com/report/kubernetes/deployment-manager)

Deployment Manager (DM) provides parameterized templates for Kubernetes resources,
such as:

* [Replicated Service](templates/replicatedservice/v1)
* [Redis](templates/redis/v1)

Templates live in ordinary Github repositories called template registries. This
Github repository contains a template registry, as well as the DM source code.

You can use DM to deploy simple configurations that use templates, such as:

* [Guestbook](examples/guestbook/guestbook.yaml)
* [Deployment Manager](examples/bootstrap/bootstrap.yaml)

A configuration is just a `YAML` file that supplies parameters. (Yes, 
you're reading that second example correctly. It uses DM to deploy itself. See
[examples/bootstrap/README.md](examples/bootstrap/README.md) for more information.)

DM runs server side, in your Kubernetes cluster, so it can tell you what types
you've instantiated there, including both primitive types and templates, what
instances you've created of a given type, and even how the instances are organized.
So, you can ask questions like:

* What Redis instances are running in this cluster?
* What Redis master and slave services are part of this Redis instance?
* What pods are part of this Redis slave?

Because DM stores its state in the cluster, not on your workstation, you can ask
those questions from any client at any time.

For more information about types, including both primitive types and templates,
see the [design document](../design/design.md#types).

Please hang out with us in
[the Slack chat room](https://kubernetes.slack.com/messages/sig-configuration/)
and/or [the Google Group](https://groups.google.com/forum/#!forum/kubernetes-sig-config)
for the Kubernetes configuration SIG. Your feedback and contributions are welcome.

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
hitting a `kubectl` proxy. To set that up:

1. Build the tool by running `make` in the deployment-manager repository.
1. Run `kubectl proxy --port=8001 --namespace=dm &` to start a proxy that lets you interact
with the Kubernetes API server through port 8001 on localhost. `dm` uses
`http://localhost:8001/api/v1/proxy/namespaces/dm/services/manager-service:manager`
as the default service address for DM.

### Using the client

The DM client, `dm`, can deploy configurations from the command line. It can also
pull templates from a template registry, generate configurations from them using
parameters supplied on the command line, and deploy the resulting configurations.

#### Deploying a configuration

`dm` can deploy a configuration from a file, or read one from `stdin`. This
command deploys the canonical Guestbook example from the examples directory:

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
deploys a redis cluster with two workers from the redis template in this repository:

```
dm deploy redis:v1
```

You can optionally supply values for template parameters on the command line,
like this:

```
dm --properties workers=3 deploy redis:v1
```

When you deploy a template directly, without a configuration, `dm` generates a
configuration from the template and any supplied parameters, and then deploys the
generated configuration.

For more information about deploying templates from a template registry or adding
types to a template registry, see [the template registry documentation](docs/templates/registry.md).

### Additional commands

`dm` makes it easy to configure a cluster from a set of predefined templates.
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
fix or feature. We use the same [development process](CONTRIBUTING.md) as the main
Kubernetes repository.

## Relationship to Google Cloud Platform
DM uses the same concepts and languages as
[Google Cloud Deployment Manager](https://cloud.google.com/deployment-manager/overview),
but creates resources in Kubernetes clusters, not in Google Cloud Platform projects.

