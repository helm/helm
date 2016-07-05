# Charts

Helm uses a packaging format called _charts_. A chart is a collection of files
that describe a related set of Kubernetes resources. A single chart
might be used to deploy something simple, like a memcached pod, or
something complex, like a full web app stack with HTTP servers,
databases, caches, and so on.

Charts are created as files laid out in a particular directory tree,
then they can be packaged into versioned archives to be deployed.

This document explains the chart format, and provides basic guidance for
building charts with Helm.

## The Chart File Structure

A chart is organized as a collection of files inside of a directory. The
directory name is the name of the chart (without versioning information). Thus,
a chart describing Wordpress would be stored in the `wordpress/` directory.

Inside of this directory, Helm will expect a structure that matches this:

```
wordpress/
  Chart.yaml        # A YAML file containing information about the chart
  LICENSE           # OPTIONAL: A plain text file containing the license for the chart
  README.md         # OPTIONAL: A human-readable README file
  values.yaml       # The default configuration values for this chart
  charts/           # OPTIONAL: A directory containing any charts upon which this chart depends.
  templates/        # OPTIONAL: A directory of templates that, when combined with values,
                    # will generate valid Kubernetes manifest files.
```

Helm will silently strip out any other files.

## The Chart.yaml File

The `Chart.yaml` file is required for a chart. It contains the following fields:

```yaml
name: The name of the chart (required)
version: A SemVer 2 version (required)
description: A single-sentence description of this project (optional)
keywords:
  - A list of keywords about this project (optional)
home: The URL of this project's home page (optional)
sources:
  - A list of URLs to source code for this project (optional)
maintainers: # (optional)
  - name: The maintainer's name (required for each maintainer)
    email: The maintainer's email (optional for each maintainer)
engine: gotpl # The name of the template engine (optional, defaults to gotpl)
```

If you are familiar with the `Chart.yaml` file format for Helm Classic, you will
notice that fields specifying dependencies have been removed. That is because
the new Chart format expresses dependencies using the `charts/` directory.

Other fields will be silently ignored.

### Charts and Versioning

Every chart must have a version number. A version must follow the
[SemVer 2](http://semver.org/) standard. Unlike Helm Classic, Kubernetes
Helm uses version numbers as release markers. Packages in repositories
are identified by name plus version.

For example, an `nginx` chart whose version field is set to `version:
1.2.3` will be named:

```
nginx-1.2.3.tgz
```

More complex SemVer 2 names are also supported, such as
`version: 1.2.3-alpha.1+ef365`. But non-SemVer names are explicitly
disallowed by the system.

**NOTE:** Whereas Helm Classic and Deployment Manager were both
very GitHub oriented when it came to charts, Kubernetes Helm does not
rely upon or require GitHub or even Git. Consequently, it does not use
Git SHAs for versioning at all.

The `version` field inside of the `Chart.yaml` is used by many of the
Helm tools, including the CLI and the Tiller server. When generating a
package, the `helm package` command will use the version that it finds
in the `Chart.yaml` as a token in the package name. The system assumes
that the version number in the chart package name matches the version number in
the `Chart.yaml`. Failure to meet this assumption will cause an error.

## Chart Dependencies

In Helm, one chart may depend on any number of other charts. These
dependencies are expressed explicitly by copying the dependency charts
into the `charts/` directory.

**Note:** The `dependencies:` section of the `Chart.yaml` from Helm
Classic has been completely removed.

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

**TIP:** _To drop a dependency into your `charts/` directory, use the
`helm fetch` command._

## Templates and Values

By default, Helm Chart templates are written in the Go template language, with the
addition of 50 or so [add-on template
functions](https://github.com/Masterminds/sprig). (In the future, Helm
may support other template languages, like Python Jinja.)

All template files are stored in a chart's `templates/` folder. When
Helm renders the charts, it will pass every file in that directory
through the template engine.

Values for the templates are supplied two ways:
  - Chart developers may supply a file called `values.yaml` inside of a
    chart. This file can contain default values.
  - Chart users may supply a YAML file that contains values. This can be
    provided on the command line with `helm install`.

When a user supplies custom values, these values will override the
values in the chart's `values.yaml` file.
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

The above example, based loosely on [https://github.com/deis/charts](https://github.com/deis/charts), is a template for a Kubernetes replication controller.
It can use the following four template values:

- `imageRegistry`: The source registry for the Docker image.
- `dockerTag`: The tag for the docker image.
- `pullPolicy`: The Kubernetes pull policy.
- `storage`: The storage backend, whose default is set to `"minio"`

All of these values are defined by the template author. Helm does not
require or dictate parameters.

### Predefined Values

The following values are pre-defined, are available to every template, and
cannot be overridden. As with all values, the names are _case
sensitive_.

- `Release.Name`: The name of the release (not the chart)
- `Release.Time`: The time the chart release was last updated. This will
  match the `Last Released` time on a Release object.
- `Release.Namespace`: The namespace the chart was released to.
- `Release.Service`: The service that conducted the release. Usually
  this is `Tiller`.
- `Chart`: The contents of the `Chart.yaml`. Thus, the chart version is
  obtainable as `Chart.Version` and the maintainers are in
  `Chart.Maintainers`.

**NOTE:** Any unknown Chart.yaml fields will be dropped. They will not
be accessible inside of the `Chart` object. Thus, Chart.yaml cannot be
used to pass arbitrarily structured data into the template.

### Values files

Considering the template in the previous section, a `values.yaml` file
that supplies the necessary values would look like this:

```yaml
imageRegistry: "quay.io/deis"
dockerTag: "latest"
pullPolicy: "alwaysPull"
storage: "s3"
```

A values file is formatted in YAML. A chart may include a default
`values.yaml` file. The Helm install command allows a user to override
values by supplying additional YAML values:

```console
$ helm install --values=myvals.yaml wordpress
```

When values are passed in this way, they will be merged into the default
values file. For example, consider a `myvals.yaml` file that looks like
this:

```yaml
storage: "gcs"
```

When this is merged with the `values.yaml` in the chart, the resulting
generated content will be:

```yaml
imageRegistry: "quay.io/deis"
dockerTag: "latest"
pullPolicy: "alwaysPull"
storage: "gcs"
```

Note that only the last field was overridden.

**NOTE:** The default values file included inside of a chart _must_ be named
`values.yaml`. But files specified on the command line can be named
anything.

### Scope, Dependencies, and Values

Values files can declare values for the top-level chart, as well as for
any of the charts that are included in that chart's `charts/` directory.
Or, to phrase it differently, a values file can supply values to the
chart as well as to any of its dependencies. For example, the
demonstration Wordpress chart above has both `mysql` and `apache` as
dependencies. The values file could supply values to all of these
components:

```yaml
title: "My Wordpress Site" # Sent to the Wordpress template

mysql:
  max_connections: 100 # Sent to MySQL
  password: "secret"

apache:
  port: 8080 # Passed to Apache
```

Charts at a higher level have access to all of the variables defined
beneath. So the wordpress chart can access `.mysql.password`. But lower
level charts cannot access things in parent charts, so MySQL will not be
able to access the `title` property. Nor, for that matter, can it access
`.apache.port`.

Values are namespaced, but namespaces are pruned. So for the Wordpress
chart, it can access the MySQL password field as `.mysql.password`. But
for the MySQL chart, the scope of the values has been reduced and the
namespace prefix removed, so it will see the password field simply as
`.password`.

#### Global Values

As of 2.0.0-Alpha.2, Helm supports special "global" value. Consider
this modified version of the previous example:

```yaml
title: "My Wordpress Site" # Sent to the Wordpress template

global:
  app: MyWordpress

mysql:
  max_connections: 100 # Sent to MySQL
  password: "secret"

apache:
  port: 8080 # Passed to Apache
```

The above adds a `global` section with the value `app: MyWordpress`.
This value is available to _all_ charts as `.Values.global.app`.

For example, the `mysql` templates may access `app` as `{{.Values.global.app}}`, and
so can the `apache` chart. Effectively, the values file above is
regenerated like this:

```yaml
title: "My Wordpress Site" # Sent to the Wordpress template

global:
  app: MyWordpress

mysql:
  global:
    app: MyWordpress
  max_connections: 100 # Sent to MySQL
  password: "secret"

apache:
  global:
    app: MyWordpress
  port: 8080 # Passed to Apache
```

This provides a way of sharing one top-level variable with all
subcharts, which is useful for things like setting `metadata` properties
like labels.

If a subchart declares a global variable, that global will be passed
_downward_ (to the subchart's subcharts), but not _upward_ to the parent
chart. There is no way for a subchart to influence the values of the
parent chart.

_Global sections are restricted to only simple key/value pairs. They do
not support nesting._

For example, the following is **illegal** and will not work:

```yaml
global:
  foo:  # It is illegal to nest an object inside of global.
    bar: baz
```

### References

When it comes to writing templates and values files, there are several
standard references that will help you out.

- [Go templates](https://godoc.org/text/template)
- [Extra template functions](https://godoc.org/github.com/Masterminds/sprig)
- [The YAML format]()

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

## Chart Repositories

A _chart repository_ is an HTTP server that houses one or more packaged
charts. While `helm` can be used to manage local chart directories, when
it comes to sharing charts, the preferred mechanism is a chart
repository.

Any HTTP server that can serve YAML files and tar files and can answer
GET requests can be used as a repository server.

Helm comes with built-in package server for developer testing (`helm
serve`). The Helm team has tested other servers, including Google Cloud
Storage with website mode enabled, and S3 with website mode enabled.

A repository is characterized primarily by the presence of a special
file called `index.yaml` that has a list of all of the packages supplied
by the repository, together with metadata that allows retrieving and
verifying those packages.

On the client side, repositories are managed with the `helm repo`
commands. However, Helm does not provide tools for uploading charts to
remote repository servers. This is because doing so would add
substantial requirements to an implementing server, and thus raise the
barrier for setting up a repository.
