# Helm/DM Developer Experience Workflows

This document outlines the individual workflows that Helm is designed to
solve.

In this document we examine several workflows that we feel are central to
the experience of deploying, managing, and building applications using
Helm. Each workflow is followed by a basic model of where the processing
of commands occurs.

### User Workflow

The user workflow is the common case. This answers stories for the "tire
kicker" and "standard user" personas.

#### Installation

Currently, the client can be used with no installation. However, installing the server side component is done like this:

```
$ helm dm install
```

- Client uses existing `kubectl` configuration to install built-in manifests.

General pattern:
```
helm dm install
```

#### Searching

```
$ helm search bar
helm:example.com/foo/bar - A basic chart
helm:example.com/foo/barracuda - A fishy chart
helm:example.com/foo/barbecue - A smoky chart
```

- Client submits the query to the manager
- Server searches the available chart repos

General pattern:
```
helm search PATTERN
```


#### Simple deployment:

```
$ helm deploy helm:example.com/foo/bar
Created wonky-panda
```

- The client sends the server a request to deploy `helm:example.com/foo/bar`.
- The server assigns a random name `wonky-panda`, fetches the chart from
  object storage, and goes about the deployment process.

General patterns:
```
helm deploy [-f CONFIG] [-n NAME] [CHART]
```


#### Find out about params:

In this operation, helm reads a chart and returns the list of parameters
that can be supplied in a template:

```
$ helm chart show helm:example.com/foo/bar
Params:
- bgcolor: The background color for the home page (hex code or HTML colors)
- title: The title of the app
```

- The client sends the request to the API server
- The API server fetches the chart, analyzes it, and returns the list of
  parameters.
  
General pattern:
```
helm chart show CHART
```


#### Generate the params for me:

In this operation, helm generates a parameter values file for the user.

```
$ helm chart params helm:example.com/foo/bar
Created: values.yaml
$ edit values.yaml
```

- The client sends the request to the server
- The server returns a stub file
- The client writes the file to disk

General pattern:
```
helm chart params CHART [CHART...]
```

#### Deploy with params:

In this operation, the user deploys a chart with an associated values
file.

```
$ helm deploy -f values.yaml helm:example.com/foo/bar 
Created taco-tuesday
```

- The client sends a request with the name of the chart and the values
  file.
- The API server generates a name (`taco-tuesday`)
- The API server fetches the request chart, sends the chart and values
  through the template resolution phase, and deploys the results.

If we allow the user to pass in a name (overriding the generated name),
then the server must first guarantee that this name is unique.

Alternately, just the values file may be specified, since it contains a reference to the base chart.

```
$ helm deploy -f values.yaml
Created taco-wednesday
```

#### Get the info about named deployment.

A deployment, as we have seen, is a named instance of a chart.
Operations that operate on these instances use the name to refer to the
instance.

```
$ helm status taco-tuesday
OK
Located at: 10.10.10.77:8080
```

- The client sends the API server a request for the status of
  `taco-tuesday`.
- The API server looks up pertinent data and returns it.
- The client displays the data.

General pattern:
```
helm status NAME
```

#### Edit and redeploy:

Redeployment is taking an existing _instance_ and changing its template
values, and then re-deploying it.

```
$ edit values.yaml
$ helm redeploy -f values.yaml taco-tuesday
Redeployed taco-tuesday
```

- The client sends the instance name and the new values to the server
- The server coalesces the values into the existing instance and then
  restarts.

General pattern:
```
helm redeploy [-f CONFIG] NAME
```


#### Delete:

```
$ helm deployment delete taco-tuesday
Destroyed taco-tuesday
```

- The client sends the DELETE instance name command
- The API server destroys the resource

General pattern:
```
helm deployment delete NAME [NAME...]
```

### Power User Features

Users familiar with the system may desire additional tools.

#### Name a deploy

```
$ helm deploy -name skinny-pigeon example.com/foo/bar
```

This follows the deployment process above. The server _must_ ensure that
the name is unique.

#### Get values for an app:
```
$ helm deployment params taco-tuesday
Stored in values.yaml
$ helm redeploy taco-tuesday values.yaml
```

- The client sends the instance name to the values endpoint
- The server returns the values file used to generate the instance.
- The client writes this to a file (or, perhaps, to STDOUT)

General pattern:
```
helm deployment params NAME [NAME...]
```

When more than one name is specified, the resulting file will contain configs for all names.

#### Get fully generated manifest files

```
$ helm deployment manifest taco-tuesday
Created manifest.yaml
```

- The client sends the instance name to the manifests endpoint
- The server returns the manifests, as generated during the
  deploy/redeploy cycle done prior.

General pattern:
```
helm deployment manifest NAME [NAME...]
```

#### Auto-detecting helm problems

```
$ helm doctor
```

- The client performs local diagnostics and diagnostics of Kubernetes and the DM server.

General pattern:
```
helm doctor
```

#### Listing all installed charts

```
$ helm chart list
helm:example.com/foo/bar#1.1.1 
helm:example.com/foo/bar#1.1.2
helm:example.com/foo/barbecue#0.1.0
```

- The client sends the server a request for all charts installed
- The server computes and responds

General pattern:
```
helm chart list
```

#### Get instances of a chart

NB: We might rename this `helm chart instances`, as that is less vague.

```
$ helm chart get helm:example.com/foo/bar 
taco-tuesday
taco-wednesday
```

- The client sends a request to the API server.
- The server responds with a list of deployment instance names.

This retrieves a shallow list, and does not inspect instances for ancestor charts.

General pattern:
```
helm chart get CHART
```

### Listing deployments

```
$ helm deployment list
skinny-pigeon
taco-tuesday
taco-wednesday
```

- The client requests a list from the server
- The server returns the list

General pattern:
```
helm deployment list
```

### Getting details of a deployment

_NB: Might not need this._

```
$ helm deployment show skinny-pigeon
DETAILS
```

### Developer Workflow

This section covers the experience of the chart developer.

In this case, when the client detects that it is working with a local
chart, it bundles the chart, and sends the entire chart, not just the
values.

```
$ helm create mychart
Created mychart/Chart.yaml
$ helm lint mychart
OK
$ helm deploy .
Uploading ./mychart as localchart/mychart-0.0.1.tgz
Created skinny-pigeon
$ helm status skinny-pigeon
OK
$ edit something
$ helm redeploy skinny-pigeon
Redeployed skinny-pigeon
```

- `helm create` and `helm lint` are client side operations
- `helm deploy`, `helm status`, and `helm redeploy` are explained above.

General pattern for create:
```
helm create [--from NAME] CHARTNAME
```

Where `NAME` will result in fetching the generated values from the cluster.

General pattern for lint:
```
helm lint PATH
```

#### Packaging and Releasing packages

Package a chart:

```
$ helm package .
Created foo-1.1.2.tgz
```

Releasing a chart:

```
$ helm release -u https://example.com/bucket ./foo-1.1.2.tgz
Uploaded to https://example.com/bucket/foo-1.1.2.tgz
```

General pattern:
```
helm package PATH
helm release [-u destination] PATH|FILE
```


### Helm Cluster Management Commands

#### Install
```
$ helm dm install
```
- Client installs using the current kubectl configuration

General pattern:
```
helm dm install
```

#### Uninstall

```
$ helm dm uninstall
```

- The client interacts with the Kubernetes API server

General pattern:
```
helm dm uninstall
```

#### Check which cluster is the current target for helm

```
$ helm dm target
API Endpoint: https://10.21.21.21/
```

- The client interacts with the local Kubernetes config and the Kubernetes API server

General pattern:
```
helm search PATTERN
```

#### View status of DM service

```
$ helm dm status
OK
```

- The client interacts with the Kubernetes API server

General pattern:
```
helm dm status
```

### Repository Configuration

#### Listing repositories

```
$ helm repo list
```

- The client request info from the server
- The server provides information about which repositories it is aware of

General pattern:
```
helm repo list
```

#### Adding credentials

```
$ helm credential add aff34... 89897a...
Created token-foo
```

- The client sends a request to the server
- The server creates a new token and returns a name

General pattern:
```
helm credential add TOKEN SECRET
```

#### Adding a repository with credentials

```
$ helm repo add -c token-foo https://example.com/charts
```

The URL is of the form `PROTO://HOST[:PORT]/BUCKET`.

General pattern:
```
helm repo add [-c TOKEN_NAME] REPO_URL
```

#### Removing repositories

```
$ helm repo rm https://example.com/charts
```

- The client sends a request to the server
- The server removes the repo from the known list

General pattern:
```
helm repo rm REPO_URL
```

#### Listing Credentials

```
$ helm credential list
token-foo: TOKEN
```

- The client requests a list of tokens
- The server returns the name and the token, but not the secret

General pattern:
```
helm credential list [PATTERN]
```

#### Removing credentials

```
$ helm credential rm token-foo
```

- The client sends a request to the server
- The server deletes the credential

General pattern:
```
helm credential rm CREDENTIAL_NAME
```
