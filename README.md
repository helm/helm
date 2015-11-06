# Deployment Manager

Deployment Manager lets you define and deploy simple declarative configuration
for your Kubernetes resources (e.g., pods, replication controllers, services, etc.).

You can also use Python or [Jinja](http://jinja.pocoo.org/) to create powerful
parameterizable abstract types called **Templates**. You can create general
abstract building blocks to reuse, like a
[Replicated Service](examples/guestbook/replicatedservice.py), or create
more concrete types like a [Redis cluster](examples/guestbook/redis.jinja).

You can find more examples of Templates and configurations in our
[examples](examples).

Deployment Manager uses the same concepts and languages as
[Google Cloud Deployment Manager](https://cloud.google.com/deployment-manager/overview),
but creates resources within your Kubernetes cluster, not on the Google Cloud Platform.

Please join us on [the Google Group](https://groups.google.com/forum/#!forum/kubernetes-sig-config) and/or in [the Slack chat room](https://kubernetes.slack.com/messages/sig-configuration/) for the
Kubernetes configuration SIG.

## Getting started

There are two ways to get started...

* The quick way simply installs Deployment Manager in your cluster using
kubectl. This is the fastest way to get started and takes only a few seconds.

* The interesting way bootstraps Deployment Manager, by building and running a
local instance on your machine, and then using it to install another instance
in your cluster. You might want to go this way if you're interested in contributing
to Deployment Manager.

Both assume that you have a Kubernetes cluster up and running, and that you can
run `kubectl` commands against it. They both also assume that that you're working
with a clone of the repository installed in the src folder of your GOPATH, per
convention.

Instructions for the quick install follow here. Instructions for bootstrapping
Deployment Manager can be found in [examples/bootstrap/README.md](examples/bootstrap/README.md).

### Quick Install

For the quick install, you're going to use `kubectl` to create the replication
controllers and services that comprise a Deployment Manager instance from a predefined
configuration file, as follows:

```
kubectl create -f install.yaml
```

That's it. You should now be able to see Deployment Manager running in your cluster
using:

```
kubectl get pod,rc,service
```

If you see replication controllers named expandybird-rc, manager-rc and resourcifier-rc
with pods that are READY, and services with corresponding names, then Deployment
Manager is up and running.

Note that you can also tear down Deployment Manager using the same file, with:

```
kubectl delete -f install.yaml
```

The easiest way to interact with Deployment Manager, now that it's up and running,
is to use a `kubectl` proxy:

```
kubectl proxy --port=8001 &
```

This command will start a proxy that lets you interact with the Kubernetes api
server through port 8001 on you local host. However, there are other ways to access
Deployment Manager. We won't go into them here, but if you know how to access
services running on Kubernetes, you should be able to use any of the supported
methods to access Deployment Manager.

### Deploying your first application (Guestbook)

Next, you're going to deploy the canonical guestbook example to your Kubernetes
cluster.

```
client --name guestbook --service=http://localhost:8001/api/v1/proxy/namespaces/default/services/manager-service:manager examples/guestbook/guestbook.yaml
```

You should now have guestbook up and running. To verify, get the list of services
running on the cluster:

```
kubectl get service
```

You should see frontend-service running. If your cluster supports external
load balancing, it will have an external IP assigned to it, and you should be 
able to navigate to it in your browser to see the guestbook in action.

## Building the container images

This project runs Deployment Manager on Kubernetes as three replicated services.
By default, prebuilt images stored in Google Container Registry are used to create
them. However, you can build your own container images and push them to your own
project in the registry. 

To build and push your own images to Google Container Registry, first set the
environment variable PROJECT to the name of a project known to gcloud. Then, run
the following command:

```
make push
```

## Design of Deployment Manager

There is a more detailed [design document](docs/design/design.md)
available.

## Status of the project

The project is still under active development, so you might run into issues. If
you do, please don't be shy about letting us know, or better yet, contributing a
fix or feature. We use the same contribution conventions as the main Kubernetes
repository.


