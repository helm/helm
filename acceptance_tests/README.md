# Helm Acceptance Tests

This directory contains the source for Helm acceptance tests.

The tests are written using [Robot Framework](https://robotframework.org/).

## System requirements

The following tools/commands are expected to be present on the base system
prior to running the tests:

- [kind](https://kind.sigs.k8s.io/)
- [kubectl](https://kubernetes.io/docs/tasks/tools/install-kubectl/)
- [python3](https://www.python.org/downloads/)
- [pip](https://pip.pypa.io/en/stable/installing/)
- [virtualenv](https://virtualenv.pypa.io/en/latest/installation/)

## Running the tests

From the root of this repo, run the following:

```
make acceptance
```

## Viewing the results

Robot creates an HTML test report describing test successes/failures.

To view the report, runt the following:

```
open .acceptance/report.html
```

Note: by default, the tests will output to the `.acceptance/` directory.
To modify this location, set the `ROBOT_OUTPUT_DIR` environment variable.

## Kubernetes integration

When testing Helm against multiple Kubernetes versions,
new test clusters are created on the fly (using `kind`),
with names in the following format:

```
helm-acceptance-test-<timestamp>-<kube_version>
```

If you wish to use an existing `kind` cluster for one
or more versions, you can set an environment variable for
a given version.

Here is an example of using an existing `kind` cluster
for Kubernetes version `1.15.0`:

```
export KIND_CLUSTER_1_15_0="helm-ac-keepalive-1.15.0"
```

A `kind` cluster can be created manually like so:

```
kind create cluster \
  --name=helm-ac-keepalive-1.15.0 \
  --image=kindest/node:v1.15.0
```

## Adding a new test case etc.

All files ending in `.robot` extension in this directory will be executed.
Add a new file describing your test, or, alternatively, add to an existing one.

Robot tests themselves are written in (mostly) plain English, but the Python
programming language can be used in order to add custom keywords etc.

Notice the [lib/](./lib/) directory - this contains Python libraries that
enable us to work with system tools such as `kind`. The file [common.py](./lib/common.py)
contains a base class called `CommandRunner` that you will likely want to
leverage when adding support for a new external tool.

The test run is wrapped by [acceptance.sh](./../scripts/acceptance.sh) -
in this file the environment is validated (i.e. check if required tools present).

sinstalled (including Robot Framework itself). If any additional Python libraries
are required for a new library, it can be appended to `ROBOT_PY_REQUIRES`.
