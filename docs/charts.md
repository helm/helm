# Charts

Helm uses a packaging format called _charts_. A chart is a collection of files
that collectively describe a set of Kubernetes resources.

## The Chart File Structure

A chart is organized as a collection of files inside of a directory. The
directory name is the name of the chart (without versioning information). Thus,
a chart describing Wordpress would be stored in the `wordpress/` directory.

Inside of this directory, Helm will expect a structure that matches this:

```
wordpress/
  Chart.yaml        # A YAML file containing information about the chart
  LICENSE           # A plain text file containing the license for the chart
  README.md         # A human-readable README file
  values.toml       # The default configuration values for this chart
  charts/           # A directory containing any charts upon which this chart depends.
  templates/        # A directory of templates that, when combined with values,
                    # will generate valid Kubernetes manifest files.
```

## The Chart.yaml File

The Chart.yaml file is required for a chart. It contains the following fields:

```yaml
name: The name of the chart (required)
version: A SemVer 2 version (required)
description: A single-sentence description of this project (optional)
keywords:
  - A list of keywords about this project
home: The URL of this project's home page (optional)
sources:
  - A list of URLs to source code for this project (optional)
maintainers:
  - name: The maintainer's name
    email: The maintainer's email
```

If you are familiar with the Chart.yaml file format for Helm Classic, you will
notice that fields specifying dependencies have been removed. That is because
the new Chart format expresses dependencies using the `charts/` directory.

## Chart Dependencies

In Helm, one chart may depend on any number of other charts. These
dependencies are expressed explicitly by copying the dependency charts
into the `charts/` directory.

For example, if the Wordpress chart depends on the Apache chart, the
Apache chart (of the correct version) is supplied in the Wordpress
chart's `charts/` directory:

```
wordpress:
  Chart.yaml
  # ...
  charts/
    apache/
      Chart.yaml
      # ...
    mysql/
      Chart.yaml
      # ...
```

The example above shows how the Wordpress chart expresses its dependency
on Apache and MySQL by including those charts inside of its `charts/`
directory.

## Templates and Values

In Helm Charts, templates are written in the Go template language, with the 
addition of 50 or so add-on template functions.

All template files are stored in a chart's `templates/` folder. When
Helm renders the charts, it will pass every file in that directory
through the template engine.

Values for the templates are supplied two ways:
  - Chart developers may supply a file called `values.toml` inside of a
    chart. This file can contain default values.
  - Chart users may supply a TOML file that contains values. This can be
    provided on the command line with `helm install`.

When a user supplies custom values, these values will override the
values in the chart's `values.toml` file.

### Template Files

Template files follow the standard conventions for writing Go templates.
An example template file might look something like this:

```yaml
apiVersion: v1
kind: ReplicationController
metadata:
  name: deis-database
  namespace: deis
  labels:
    heritage: deis
spec:
  replicas: 1
  selector:
    app: deis-database
  template:
    metadata:
      labels:
        app: deis-database
    spec:
      serviceAccount: deis-database
      containers:
        - name: deis-database
          image: {{.imageRegistry}}/postgres:{{.dockerTag}}
          imagePullPolicy: {{.pullPolicy}}
          ports:
            - containerPort: 5432
          env:
            - name: DATABASE_STORAGE
              value: {{default "minio" .storage}}
```

The above example, based loosely on [https://github.com/deis/charts](the
chart for Deis), is a template for a Kubernetes replication controller.
It can use the following four template values:

- `imageRegistry`: The source registry for the Docker image.
- `dockerTag`: The tag for the docker image.
- `pullPolicy`: The Kubernetes pull policy.
- `storage`: The storage backend, whose default is set to `"minio"`

All of these values are defined by the template author. Helm does not
require or dictate parameters.

### Values files

Considering the template in the previous section, a `values.toml` file
that supplies the necessary values would look like this:

```toml
imageRegistry = "quay.io/deis"
dockerTag = "latest"
pullPolicy = "alwaysPull"
storage = "s3"
```

When a chart includes dependency charts, values can be supplied to those
charts using TOML tables:

```toml
imageRegistry = "quay.io/deis"
dockerTag = "latest"
pullPolicy = "alwaysPull"
storage = "s3"

[router]
hostname = "example.com"
```

In the above example, the value of `hostname` will be passed to a chart
named `router` (if it exists) in the `charts/` directory.

### References
- [Go templates](https://godoc.org/text/template)
- [Extra template functions](https://godoc.org/github.com/Masterminds/sprig)
- [The TOML format](https://github.com/toml-lang/toml)

## Using Helm to Manage Charts

The `helm` tool has several commands for working with charts.

It can create a new chart for you:

```console
$ helm create mychart
Created mychart/
```

Once you have edited a chart, `helm` can package it into a chart archive
for you:

```console
$ helm package mychart
Archived mychart-0.1.-.tgz
```

You can also use `helm` to help you find issues with your chart's
formatting or information:

```console
$ helm lint mychart
No issues found
```
