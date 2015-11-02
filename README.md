# Deployment Manager

Deployment Manager lets you define and deploy simple declarative configuration
for your Kubernetes resources.

You can also use Python or [Jinja](http://jinja.pocoo.org/) to create powerful
parameterizable abstract types called **Templates**. You can create general
abstract building blocks to reuse, like a
[Replicated Service](examples/guestbook/replicatedservice.py), or create
more concrete types like a [Redis cluster](examples/guestbook/redis.jinja).

You can find more examples of Templates and configurations in our
[examples](examples).

Deployment Manager uses the same concepts and languages as
[Google Cloud Deployment Manager](https://cloud.google.com/deployment-manager/overview),
but works directly within your Kubernetes cluster.


## Getting started

For the following steps, it is assumed that you have a Kubernetes cluster up
and running, and that you can run kubectl commands against it. It is also
assumed that you're working with a clone of the repository installed in the src
folder of your GOPATH, and that your PATH contains $GOPATH/bin, per convention.

Since Deployment Manager uses Python and will be running locally on your
machine, you will first need to make sure the necessary Python packages are
installed. This assumes that you have already installed the pip package
management system on your machine.

```
pip install -r expandybird/requirements.txt
```

Next, you'll build and install the binaries, and bootstrap Deployment Manager
into the cluster. Finally, you'll deploy an example application on the
cluster using Deployment Manager.

### Building and installing the binaries

In this step, you're going to build and install the Deployment Manager binaries.
You can do this by running make in the repository root.

```
make
```

### Bootstrapping Deployment Manager

In this step, you're going to bootstrap Deployment Manager into the cluster.

Next, start the three Deployment Manager binaries on localhost using the supplied
bootstrap script.

```
./examples/bootstrap/bootstrap.sh
```

The script starts the following binaries:
* manager (frontend service) running on port 8080
* expandybird (expands templates) running on port 8081
* resourcifier (reifies primitive Kubernetes resources) running on port 8082

It also starts kubectl proxy on port 8001.

Next, use the Deployment Manager running on localhost to deploy itself onto the
cluster using the supplied command line tool and template.

```
client --name test --service=http://localhost:8080 examples/bootstrap/bootstrap.yaml
```

You should now have Deployment Manager running on your cluster, and it should be
visible using kubectl (kubectl get pod,rc,service).

### Deploying your first application (Guestbook)

In this step, you're going to deploy the canonical guestbook example to your
Kubernetes cluster.

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
able to navigate to it to see the guestbook in action.

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
