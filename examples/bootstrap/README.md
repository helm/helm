# Bootstrapping Deployment Manager

Welcome to the bootstrap example. The instructions below will step you through
the process of building and running a local instance of DM on your local machine,
and then using it to deploy another instance of DM in your cluster.

This example provides insights into how DM works, and is recommended for anyone
interested in contributing to the project.

## Prerequisites

Before you can bootstrap DM, the following prerequisites must be satisfied.

### Kubernetes cluster and go configuration

1. Make sure your Kubernetes cluster is up and running, and that you can run
`kubectl` commands against it.
1. Clone this repository into the src folder of your GOPATH, if you haven't already.
1. Make sure your PATH contains `$GOPATH/bin`.

### Installing required python packages

Since Deployment Manager uses Python and will be running locally on your
machine, you need to make sure the necessary Python packages are installed. This
step assumes that you have already installed the pip package management system
on your machine.

```
pip install -r expandybird/requirements.txt
```

Note: depending on how you installed python and pip, you may need to use `sudo`
for this command.

## Bootstrapping Deployment Manager

With the prerequisites satisfied, you're ready to bootstrap DM.

### Building and installing the binaries

First, you're going to build and install the DM binaries. You can do this by
running make in the repository root.

```
make
```

### Start Deployment Manager on localhost

Next, start the three DM binaries on localhost using the supplied bootstrap script.

```
./examples/bootstrap/bootstrap.sh
```

The script starts the following binaries:
* manager (frontend service) running on port 8080
* expandybird (expands templates) running on port 8081
* resourcifier (reifies primitive Kubernetes resources) running on port 8082

It also starts kubectl proxy on port 8001.

### Deploy Deployment Manager into your cluster

Finally, use the DM running on localhost to deploy another instance of DM onto
the cluster using `dm` and the supplied template. Note that you are using the 
`--service` flag to point `dm` to the instance of DM running on localhost, rather
than to an instance of DM running in the cluster through `kubectl proxy`, which
is the default.

```
dm --service=http://localhost:8080 deploy examples/bootstrap/bootstrap.yaml
```

You now have Deployment Manager running on your cluster. You can see it running
using `kubectl`, as described in the top level [README.md](../../README.md).
