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
a chart describing WordPress would be stored in the `wordpress/` directory.

Inside of this directory, Helm will expect a structure that matches this:

```
wordpress/
  Chart.yaml          # A YAML file containing information about the chart
  LICENSE             # OPTIONAL: A plain text file containing the license for the chart
  README.md           # OPTIONAL: A human-readable README file
  requirements.yaml   # OPTIONAL: A YAML file listing dependencies for the chart
  values.yaml         # The default configuration values for this chart
  charts/             # OPTIONAL: A directory containing any charts upon which this chart depends.
  templates/          # OPTIONAL: A directory of templates that, when combined with values,
                      # will generate valid Kubernetes manifest files.
  templates/NOTES.txt # OPTIONAL: A plain text file containing short usage notes
```

Helm reserves use of the `charts/` and `templates/` directories, and of
the listed file names. Other files will be left as they are.

While the `charts` and `templates` directories are optional there must be at least one chart dependency or template file for the chart to be valid.

## The Chart.yaml File

The `Chart.yaml` file is required for a chart. It contains the following fields:

```yaml
name: The name of the chart (required)
version: A SemVer 2 version (required)
kubeVersion: A SemVer range of compatible Kubernetes versions (optional)
description: A single-sentence description of this project (optional)
keywords:
  - A list of keywords about this project (optional)
home: The URL of this project's home page (optional)
sources:
  - A list of URLs to source code for this project (optional)
maintainers: # (optional)
  - name: The maintainer's name (required for each maintainer)
    email: The maintainer's email (optional for each maintainer)
    url: A URL for the maintainer (optional for each maintainer)
engine: gotpl # The name of the template engine (optional, defaults to gotpl)
icon: A URL to an SVG or PNG image to be used as an icon (optional).
appVersion: The version of the app that this contains (optional). This needn't be SemVer.
deprecated: Whether this chart is deprecated (optional, boolean)
tillerVersion: The version of Tiller that this chart requires. This should be expressed as a SemVer range: ">2.0.0" (optional)
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

### The appVersion field

Note that the `appVersion` field is not related to the `version` field. It is
a way of specifying the version of the application. For example, the `drupal`
chart may have an `appVersion: 8.2.1`, indicating that the version of Drupal
included in the chart (by default) is `8.2.1`. This field is informational, and
has no impact on chart version calculations.

### Deprecating Charts

When managing charts in a Chart Repository, it is sometimes necessary to
deprecate a chart. The optional `deprecated` field in `Chart.yaml` can be used
to mark a chart as deprecated. If the **latest** version of a chart in the
repository is marked as deprecated, then the chart as a whole is considered to
be deprecated. The chart name can later be reused by publishing a newer version
that is not marked as deprecated. The workflow for deprecating charts, as
followed by the [kubernetes/charts](https://github.com/kubernetes/charts)
project is:
  - Update chart's `Chart.yaml` to mark the chart as deprecated, bumping the
  version
  - Release the new chart version in the Chart Repository
  - Remove the chart from the source repository (e.g. git)

## Chart LICENSE, README and NOTES

Charts can also contain files that describe the installation, configuration, usage and license of a
chart. A README for a chart should be formatted in Markdown (README.md), and should generally
contain:

- A description of the application or service the chart provides
- Any prerequisites or requirements to run the chart
- Descriptions of options in `values.yaml` and default values
- Any other information that may be relevant to the installation or configuration of the chart

The chart can also contain a short plain text `templates/NOTES.txt` file that will be printed out
after installation, and when viewing the status of a release. This file is evaluated as a
[template](#templates-and-values), and can be used to display usage notes, next steps, or any other
information relevant to a release of the chart. For example, instructions could be provided for
connecting to a database, or accessing a web UI. Since this file is printed to STDOUT when running
`helm install` or `helm status`, it is recommended to keep the content brief and point to the README
for greater detail.

## Chart Dependencies

In Helm, one chart may depend on any number of other charts. 
These dependencies can be dynamically linked through the `requirements.yaml`
file or brought in to the `charts/` directory and managed manually. 

Although manually managing your dependencies has a few advantages some teams need,
the preferred method of declaring dependencies is by using a
`requirements.yaml` file inside of your chart.

**Note:** The `dependencies:` section of the `Chart.yaml` from Helm
Classic has been completely removed.


### Managing Dependencies with `requirements.yaml`

A `requirements.yaml` file is a simple file for listing your
dependencies.

```yaml
dependencies:
  - name: apache
    version: 1.2.3
    repository: http://example.com/charts
  - name: mysql
    version: 3.2.1
    repository: http://another.example.com/charts
```

- The `name` field is the name of the chart you want.
- The `version` field is the version of the chart you want.
- The `repository` field is the full URL to the chart repository. Note
  that you must also use `helm repo add` to add that repo locally.

Once you have a dependencies file, you can run `helm dependency update`
and it will use your dependency file to download all the specified
charts into your `charts/` directory for you.

```console
$ helm dep up foochart
Hang tight while we grab the latest from your chart repositories...
...Successfully got an update from the "local" chart repository
...Successfully got an update from the "stable" chart repository
...Successfully got an update from the "example" chart repository
...Successfully got an update from the "another" chart repository
Update Complete. Happy Helming!
Saving 2 charts
Downloading apache from repo http://example.com/charts
Downloading mysql from repo http://another.example.com/charts
```

When `helm dependency update` retrieves charts, it will store them as
chart archives in the `charts/` directory. So for the example above, one
would expect to see the following files in the charts directory:

```
charts/
  apache-1.2.3.tgz
  mysql-3.2.1.tgz
```

Managing charts with `requirements.yaml` is a good way to easily keep
charts updated, and also share requirements information throughout a
team.

#### Alias field in requirements.yaml

In addition to the other fields above, each requirements entry may contain
the optional field `alias`.

Adding an alias for a dependency chart would put
a chart in dependencies using alias as name of new dependency.

One can use `alias` in cases where they need to access a chart
with other name(s).

```yaml
# parentchart/requirements.yaml
dependencies:
  - name: subchart
    repository: http://localhost:10191
    version: 0.1.0
    alias: new-subchart-1
  - name: subchart
    repository: http://localhost:10191
    version: 0.1.0
    alias: new-subchart-2
  - name: subchart
    repository: http://localhost:10191
    version: 0.1.0
```

In the above example we will get 3 dependencies in all for `parentchart`
```
subchart
new-subchart-1
new-subchart-2
```

The manual way of achieving this is by copy/pasting the same chart in the
`charts/` directory multiple times with different names.

#### Tags and Condition fields in requirements.yaml

In addition to the other fields above, each requirements entry may contain
the optional fields `tags` and `condition`.

All charts are loaded by default. If `tags` or `condition` fields are present,
they will be evaluated and used to control loading for the chart(s) they are applied to.

Condition - The condition field holds one or more YAML paths (delimited by commas).
If this path exists in the top parent's values and resolves to a boolean value,
the chart will be enabled or disabled based on that boolean value.  Only the first
valid path found in the list is evaluated and if no paths exist then the condition has no effect.

Tags - The tags field is a YAML list of labels to associate with this chart.
In the top parent's values, all charts with tags can be enabled or disabled by
specifying the tag and a boolean value.

````
# parentchart/requirements.yaml
dependencies:
      - name: subchart1
        repository: http://localhost:10191
        version: 0.1.0
        condition: subchart1.enabled, global.subchart1.enabled
        tags:
          - front-end
          - subchart1

      - name: subchart2
        repository: http://localhost:10191
        version: 0.1.0
        condition: subchart2.enabled,global.subchart2.enabled
        tags:
          - back-end
          - subchart2

````
````
# parentchart/values.yaml

subchart1:
  enabled: true
tags:
  front-end: false
  back-end: true
````

In the above example all charts with the tag `front-end` would be disabled but since the
`subchart1.enabled` path evaluates to 'true' in the parent's values, the condition will override the
`front-end` tag and `subchart1` will be enabled.  

Since `subchart2` is tagged with `back-end` and that tag evaluates to `true`, `subchart2` will be
enabled. Also notes that although `subchart2` has a condition specified in `requirements.yaml`, there
is no corresponding path and value in the parent's values so that condition has no effect.  

##### Using the CLI with Tags and Conditions

The `--set` parameter can be used as usual to alter tag and condition values.

````
helm install --set tags.front-end=true --set subchart2.enabled=false

````

##### Tags and Condition Resolution


  * **Conditions (when set in values) always override tags.** The first condition
    path that exists wins and subsequent ones for that chart are ignored.
  * Tags are evaluated as 'if any of the chart's tags are true then enable the chart'.
  * Tags and conditions values must be set in the top parent's values.
  * The `tags:` key in values must be a top level key. Globals and nested `tags:` tables
    are not currently supported.

#### Importing Child Values via requirements.yaml

In some cases it is desirable to allow a child chart's values to propagate to the parent chart and be 
shared as common defaults. An additional benefit of using the `exports` format is that it will enable future 
tooling to introspect user-settable values.

The keys containing the values to be imported can be specified in the parent chart's `requirements.yaml` file 
using a YAML list. Each item in the list is a key which is imported from the child chart's `exports` field. 

To import values not contained in the `exports` key, use the [child-parent](#using-the-child-parent-format) format.
Examples of both formats are described below.

##### Using the exports format

If a child chart's `values.yaml` file contains an `exports` field at the root, its contents may be imported 
directly into the parent's values by specifying the keys to import as in the example below:

```yaml
# parent's requirements.yaml file
    ...
    import-values:
      - data
```
```yaml
# child's values.yaml file
...
exports:
  data:
    myint: 99
```

Since we are specifying the key `data` in our import list, Helm looks in the `exports` field of the child 
chart for `data` key and imports its contents. 

The final parent values would contain our exported field:

```yaml
# parent's values file
...
myint: 99

```

Please note the parent key `data` is not contained in the parent's final values. If you need to specify the 
parent key, use the 'child-parent' format. 

##### Using the child-parent format

To access values that are not contained in the `exports` key of the child chart's values, you will need to 
specify the source key of the values to be imported (`child`) and the destination path in the parent chart's 
values (`parent`).

The `import-values` in the example below instructs Helm to take any values found at `child:` path and copy them 
to the parent's values at the path specified in `parent:`

```yaml
# parent's requirements.yaml file
dependencies:
  - name: subchart1
    repository: http://localhost:10191
    version: 0.1.0
    ...
    import-values:
      - child: default.data
        parent: myimports
```
In the above example, values found at `default.data` in the subchart1's values will be imported
to the `myimports` key in the parent chart's values as detailed below: 

```yaml
# parent's values.yaml file

myimports:
  myint: 0
  mybool: false
  mystring: "helm rocks!"
  
```
```yaml
# subchart1's values.yaml file

default:
  data:
    myint: 999
    mybool: true
    
```
The parent chart's resulting values would be:

```yaml
# parent's final values

myimports:
  myint: 999
  mybool: true
  mystring: "helm rocks!"

```

The parent's final values now contains the `myint` and `mybool` fields imported from subchart1.

### Managing Dependencies manually via the `charts/` directory

If more control over dependencies is desired, these dependencies can
be expressed explicitly by copying the dependency charts into the
`charts/` directory.

A dependency can be either a chart archive (`foo-1.2.3.tgz`) or an
unpacked chart directory. But its name cannot start with `_` or `.`.
Such files are ignored by the chart loader.

For example, if the WordPress chart depends on the Apache chart, the
Apache chart (of the correct version) is supplied in the WordPress
chart's `charts/` directory:

```
wordpress:
  Chart.yaml
  requirements.yaml
  # ...
  charts/
    apache/
      Chart.yaml
      # ...
    mysql/
      Chart.yaml
      # ...
```

The example above shows how the WordPress chart expresses its dependency
on Apache and MySQL by including those charts inside of its `charts/`
directory.

**TIP:** _To drop a dependency into your `charts/` directory, use the
`helm fetch` command_

### Operational aspects of using dependencies

The above sections explain how to specify chart dependencies, but how does this affect
chart installation using `helm install` and `helm upgrade`?

Suppose that a chart named "A" creates the following Kubernetes objects

- namespace "A-Namespace"
- statefulset "A-StatefulSet"
- service "A-Service"

Furthermore, A is dependent on chart B that creates objects

- namespace "B-Namespace"
- replicaset "B-ReplicaSet"
- service "B-Service"

After installation/upgrade of chart A a single Helm release is created/modified. The release will 
create/update all of the above Kubernetes objects in the following order:

- A-Namespace
- B-Namespace
- A-StatefulSet
- B-ReplicaSet
- A-Service
- B-Service

This is because when Helm installs/upgrades charts, 
the Kubernetes objects from the charts and all its dependencies are 

- aggregrated into a single set; then 
- sorted by type followed by name; and then 
- created/updated in that order. 

Hence a single release is created with all the objects for the chart and its dependencies.

The install order of Kubernetes types is given by the enumeration InstallOrder in kind_sorter.go 
(see [the Helm source file](https://github.com/kubernetes/helm/blob/master/pkg/tiller/kind_sorter.go#L26)).

## Templates and Values

Helm Chart templates are written in the
[Go template language](https://golang.org/pkg/text/template/), with the
addition of 50 or so add-on template
functions [from the Sprig library](https://github.com/Masterminds/sprig) and a
few other [specialized functions](charts_tips_and_tricks.md).

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

Template files follow the standard conventions for writing Go templates
(see [the text/template Go package documentation](https://golang.org/pkg/text/template/)
for details).
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
          image: {{.Values.imageRegistry}}/postgres:{{.Values.dockerTag}}
          imagePullPolicy: {{.Values.pullPolicy}}
          ports:
            - containerPort: 5432
          env:
            - name: DATABASE_STORAGE
              value: {{default "minio" .Values.storage}}
```

The above example, based loosely on [https://github.com/deis/charts](https://github.com/deis/charts), is a template for a Kubernetes replication controller.
It can use the following four template values (usually defined in a
`values.yaml` file):

- `imageRegistry`: The source registry for the Docker image.
- `dockerTag`: The tag for the docker image.
- `pullPolicy`: The Kubernetes pull policy.
- `storage`: The storage backend, whose default is set to `"minio"`

All of these values are defined by the template author. Helm does not
require or dictate parameters.

To see many working charts, check out the [Kubernetes Charts
project](https://github.com/kubernetes/charts)

### Predefined Values

Values that are supplied via a `values.yaml` file (or via the `--set`
flag) are accessible from the `.Values` object in a template. But there
are other pre-defined pieces of data you can access in your templates.

The following values are pre-defined, are available to every template, and
cannot be overridden. As with all values, the names are _case
sensitive_.

- `Release.Name`: The name of the release (not the chart)
- `Release.Time`: The time the chart release was last updated. This will
  match the `Last Released` time on a Release object.
- `Release.Namespace`: The namespace the chart was released to.
- `Release.Service`: The service that conducted the release. Usually
  this is `Tiller`.
- `Release.IsUpgrade`: This is set to true if the current operation is an upgrade or rollback.
- `Release.IsInstall`: This is set to true if the current operation is an
  install.
- `Release.Revision`: The revision number. It begins at 1, and increments with
  each `helm upgrade`.
- `Chart`: The contents of the `Chart.yaml`. Thus, the chart version is
  obtainable as `Chart.Version` and the maintainers are in
  `Chart.Maintainers`.
- `Files`: A map-like object containing all non-special files in the chart. This
  will not give you access to templates, but will give you access to additional
  files that are present (unless they are excluded using `.helmignore`). Files can be
  accessed using `{{index .Files "file.name"}}` or using the `{{.Files.Get name}}` or
  `{{.Files.GetString name}}` functions. You can also access the contents of the file
  as `[]byte` using `{{.Files.GetBytes}}`
- `Capabilities`: A map-like object that contains information about the versions
  of Kubernetes (`{{.Capabilities.KubeVersion}}`, Tiller
  (`{{.Capabilities.TillerVersion}}`, and the supported Kubernetes API versions
  (`{{.Capabilities.APIVersions.Has "batch/v1"`)

**NOTE:** Any unknown Chart.yaml fields will be dropped. They will not
be accessible inside of the `Chart` object. Thus, Chart.yaml cannot be
used to pass arbitrarily structured data into the template. The values
file can be used for that, though.

### Values files

Considering the template in the previous section, a `values.yaml` file
that supplies the necessary values would look like this:

```yaml
imageRegistry: "quay.io/deis"
dockerTag: "latest"
pullPolicy: "Always"
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
pullPolicy: "Always"
storage: "gcs"
```

Note that only the last field was overridden.

**NOTE:** The default values file included inside of a chart _must_ be named
`values.yaml`. But files specified on the command line can be named
anything.

**NOTE:** If the `--set` flag is used on `helm install` or `helm upgrade`, those
values are simply converted to YAML on the client side.

**NOTE:** If any required entries in the values file exist, they can be declared
as required in the chart template by using the ['required' function](charts_tips_and_tricks.md)

Any of these values are then accessible inside of templates using the
`.Values` object:

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
          image: {{.Values.imageRegistry}}/postgres:{{.Values.dockerTag}}
          imagePullPolicy: {{.Values.pullPolicy}}
          ports:
            - containerPort: 5432
          env:
            - name: DATABASE_STORAGE
              value: {{default "minio" .Values.storage}}

```

### Scope, Dependencies, and Values

Values files can declare values for the top-level chart, as well as for
any of the charts that are included in that chart's `charts/` directory.
Or, to phrase it differently, a values file can supply values to the
chart as well as to any of its dependencies. For example, the
demonstration WordPress chart above has both `mysql` and `apache` as
dependencies. The values file could supply values to all of these
components:

```yaml
title: "My WordPress Site" # Sent to the WordPress template

mysql:
  max_connections: 100 # Sent to MySQL
  password: "secret"

apache:
  port: 8080 # Passed to Apache
```

Charts at a higher level have access to all of the variables defined
beneath. So the WordPress chart can access the MySQL password as
`.Values.mysql.password`. But lower level charts cannot access things in
parent charts, so MySQL will not be able to access the `title` property. Nor,
for that matter, can it access `apache.port`.

Values are namespaced, but namespaces are pruned. So for the WordPress
chart, it can access the MySQL password field as `.Values.mysql.password`. But
for the MySQL chart, the scope of the values has been reduced and the
namespace prefix removed, so it will see the password field simply as
`.Values.password`.

#### Global Values

As of 2.0.0-Alpha.2, Helm supports special "global" value. Consider
this modified version of the previous example:

```yaml
title: "My WordPress Site" # Sent to the WordPress template

global:
  app: MyWordPress

mysql:
  max_connections: 100 # Sent to MySQL
  password: "secret"

apache:
  port: 8080 # Passed to Apache
```

The above adds a `global` section with the value `app: MyWordPress`.
This value is available to _all_ charts as `.Values.global.app`.

For example, the `mysql` templates may access `app` as `{{.Values.global.app}}`, and
so can the `apache` chart. Effectively, the values file above is
regenerated like this:

```yaml
title: "My WordPress Site" # Sent to the WordPress template

global:
  app: MyWordPress

mysql:
  global:
    app: MyWordPress
  max_connections: 100 # Sent to MySQL
  password: "secret"

apache:
  global:
    app: MyWordPress
  port: 8080 # Passed to Apache
```

This provides a way of sharing one top-level variable with all
subcharts, which is useful for things like setting `metadata` properties
like labels.

If a subchart declares a global variable, that global will be passed
_downward_ (to the subchart's subcharts), but not _upward_ to the parent
chart. There is no way for a subchart to influence the values of the
parent chart.

Also, global variables of parent charts take precedence over the global variables from subcharts.

### References

When it comes to writing templates and values files, there are several
standard references that will help you out.

- [Go templates](https://godoc.org/text/template)
- [Extra template functions](https://godoc.org/github.com/Masterminds/sprig)
- [The YAML format](http://yaml.org/spec/)

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

## Chart Starter Packs

The `helm create` command takes an optional `--starter` option that lets you
specify a "starter chart".

Starters are just regular charts, but are located in `$HELM_HOME/starters`.
As a chart developer, you may author charts that are specifically designed
to be used as starters. Such charts should be designed with the following
considerations in mind:

- The `Chart.yaml` will be overwritten by the generator.
- Users will expect to modify such a chart's contents, so documentation
  should indicate how users can do so.
- All occurences of `<CHARTNAME>` will be replaced with the specified chart
  name so that starter charts can be used as templates.

Currently the only way to add a chart to `$HELM_HOME/starters` is to manually
copy it there. In your chart's documentation, you may want to explain that
process.
