# Bootstrapping Deployment Manager

Welcome to the bootstrap example. The instructions below will step you through
the process of building and running a local instance of Deployment Manager on
your local machine, and then using it to deploy another instance in your cluster.

This example provides insights into how Deployment Manager works, and is
recommended for anyone interested in contributing to the project.

The instructions below assume that you have a Kubernetes cluster up and running,
and that you can run `kubectl` commands against it. They also assume that that
you're working with a clone of the repository installed in the src folder of your 
GOPATH and that your PATH contains `$GOPATH/bin`, per convention.

## Installing required python packages

Since Deployment Manager uses Python and will be running locally on your
machine, you will first need to make sure the necessary Python packages are
installed. This assumes that you have already installed the pip package
management system on your machine.

```
sudo pip install -r expandybird/requirements.txt
```

## Building and installing the binaries

Next, you're going to build and install the Deployment Manager binaries. You can
do this by running make in the repository root.

```
make
```

## Bootstrapping Deployment Manager

Now, you're ready to bootstrap Deployment Manager into the cluster.

### Start Deployment Manager on localhost

First, start the three Deployment Manager binaries on localhost using the supplied
bootstrap script.

```
./examples/bootstrap/bootstrap.sh
```

The script starts the following binaries:
* manager (frontend service) running on port 8080
* expandybird (expands templates) running on port 8081
* resourcifier (reifies primitive Kubernetes resources) running on port 8082

It also starts kubectl proxy on port 8001.

### Deploy Deployment Manager into your cluster

Next, use the Deployment Manager running on localhost to deploy itself onto the
cluster using the supplied command line tool and template.

```
client --name test --service=http://localhost:8080 examples/bootstrap/bootstrap.yaml
```

You should now have Deployment Manager running on your cluster, and it should be
visible using kubectl (kubectl get pod,rc,service).


