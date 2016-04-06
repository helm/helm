# Pushing Helm

This details the requirements and steps for doing a `helm` push.

## Prerequisites

In order to build and push `helm`, you must:

* be an editor or owner on the GCP project `kubernetes-helm`
* have `docker` installed and runnable in your current environment
* have `gcloud` installed
* have `gsutil` installed

## Pushing

To build and push the service containers and the client binaries for all 
supported platforms and architectures, checkout the branch and tag you intend to release, 
and then run the following:

```
$ DOCKER_PROJECT=kubernetes-helm make push
```
