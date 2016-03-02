# Pushing DM

This details the requirements and steps for doing a DM push.

## Prerequisites

In order to build and push DM, you must:

* be an editor or owner on the GCP project `dm-k8s-prod`
* have `docker` installed and runnable in your current environment
* have `gcloud` installed
* have `gsutil` installed

## Pushing

To build and push the service containers:

```
$ cd ${GOPATH}/src/github.com/kubernetes/deployment-manager
$ export PROJECT=dm-k8s-prod
$ make push
```

To push the client binaries, run the following for both Mac OS X and Linux
environments:

```
$ hack/dm-push.sh
```

