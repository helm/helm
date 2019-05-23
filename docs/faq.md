# Frequently Asked Questions

This page provides help with the most common questions about Helm.

**We'd love your help** making this document better. To add, correct, or remove
information, [file an issue](https://github.com/helm/helm/issues) or
send us a pull request.

## Changes since Helm 2

Here's an exhaustive list of all the major changes introduced in Helm 3.

### Removal of Tiller

During the Helm 2 development cycle, we introduced Tiller. Tiller played an important role for teams working on a shared
cluster - it made it possible for multiple different operators to interact with the same set of releases.

With role-based access controls (RBAC) enabled by default in Kubernetes 1.6, locking down Tiller for use in a production
scenario became more difficult to manage. Due to the vast number of possible security policies, our stance was to
provide a permissive default configuration. This allowed first-time users to start experimenting with Helm and
Kubernetes without having to dive headfirst into the security controls. Unfortunately, this permissive configuration
could grant a user a broad range of permissions they weren’t intended to have. DevOps and SREs had to learn additional
operational steps when installing Tiller into a multi-tenant cluster.

After hearing how community members were using Helm in certain scenarios, we found that Tiller’s release management
system did not need to rely upon an in-cluster operator to maintain state or act as a central hub for Helm release
information. Instead, we could simply fetch information from the Kubernetes API server, render the Charts client-side,
and store a record of the installation in Kubernetes.

Tiller’s primary goal could be accomplished without Tiller, so one of the first decisions we made regarding Helm 3 was
to completely remove Tiller.

With Tiller gone, the security model for Helm is radically simplified. Helm 3 now supports all the modern security,
identity, and authorization features of modern Kubernetes. Helm’s permissions are evaluated using your [kubeconfig file](https://kubernetes.io/docs/concepts/configuration/organize-cluster-access-kubeconfig/).
Cluster administrators can restrict user permissions at whatever granularity they see fit. Releases are still recorded
in-cluster, and the rest of Helm’s functionality remains.

### Release Names are now scoped to the Namespace

With the removal of Tiller, the information about each release had to go somewhere. In Helm 2, this was stored in the
same namespace as Tiller. In practice, this meant that once a name was used by a release, no other release could use
that same name, even if it was deployed in a different namespace.

In Helm 3, release information about a particular release is now stored in the same namespace as the release itself.
This means that users can now `helm install wordpress stable/wordpress` in two separate namespaces, and each can be
referred with `helm list` by changing the current namespace context.

### Go import path changes

In Helm 3, Helm switched the Go import path over from `k8s.io/helm` to `helm.sh/helm`. If you intend
to upgrade to the Helm 3 Go client libraries, make sure to change your import paths.

### Capabilities

The `.Capabilities` built-in object available during the rendering stage has been simplified.

[Built-in Objects](chart_template_guide/builtin_objects.md)

### Validating Chart Values with JSONSchema

A JSON Schema can now be imposed upon chart values. This ensures that values provided by the user follow the schema
laid out by the chart maintainer, providing better error reporting when the user provides an incorrect set of values for
a chart.

Validation occurs when any of the following commands are invoked:

* `helm install`
* `helm upgrade`
* `helm template`
* `helm lint`

See the documentation on [Schema files](charts.md#schema-files) for more information.

### Consolidation of requirements.yaml into Chart.yaml

The Chart dependency management system moved from requirements.yaml and requirements.lock to Chart.yaml and Chart.lock,
meaning that charts that relied on the `helm dependency` subcommands will need some tweaking to work in Helm 3.

In Helm 2, this is how a requirements.yaml looked:

```
dependencies:
- name: mariadb
  version: 5.x.x
  repository: https://kubernetes-charts.storage.googleapis.com/
  condition: mariadb.enabled
  tags:
    - database
```

In Helm 3, the dependency is expressed the same way, but now from your Chart.yaml:

```
dependencies:
- name: mariadb
  version: 5.x.x
  repository: https://kubernetes-charts.storage.googleapis.com/
  condition: mariadb.enabled
  tags:
    - database
```

Charts are still downloaded and placed in the charts/ directory, so subcharts vendored into the charts/ directory will continue to work without modification.

### Name (or --generate-name) is now required on install

In Helm 2, if no name was provided, an auto-generated name would be given. In production, this proved to be more of a
nuisance than a helpful feature. In Helm 3, Helm will throw an error if no name is provided with `helm install`.

For those who still wish to have a name auto-generated for you, you can use the `--generate-name` flag to create one for
you.

### Pushing Charts to OCI Registries

At a high level, a Chart Repository is a location where Charts can be stored and shared. The Helm client packs and ships
Helm Charts to a Chart Repository. Simply put, a Chart Repository is a basic HTTP server that houses an index.yaml file
and some packaged charts.

While there are several benefits to the Chart Repository API meeting the most basic storage requirements, a few
drawbacks have started to show:

- Chart Repositories have a very hard time abstracting most of the security implementations required in a production environment. Having a standard API for authentication and authorization is very important in production scenarios.
- Helm’s Chart provenance tools used for signing and verifying the integrity and origin of a chart are an optional piece of the Chart publishing process.
- In multi-tenant scenarios, the same Chart can be uploaded by another tenant, costing twice the storage cost to store the same content. Smarter chart repositories have been designed to handle this, but it’s not a part of the formal specification.
- Using a single index file for search, metadata information, and fetching Charts has made it difficult or clunky to design around in secure multi-tenant implementations.

Docker’s Distribution project (also known as Docker Registry v2) is the successor to the Docker Registry project. Many
major cloud vendors have a product offering of the Distribution project, and with so many vendors offering the same
product, the Distribution project has benefited from many years of hardening, security best practices, and
battle-testing.

Please have a look at `helm help chart` and `helm help registry` for more information on how to package a chart and
push it to a Docker registry.

### Removal of helm serve

`helm serve` ran a local Chart Repository on your machine for development purposes. However, it didn't receive much
uptake as a development tool and had numerous issues with its design. In the end, we decided to remove it and split it
out as a plugin.

### Library chart support

Helm 3 supports a class of chart called a “library chart”. This is a chart that is shared by other charts, but does not
create any release artifacts of its own. A library chart’s templates can only declare `define` elements. Globally scoped
non-`define` content is simply ignored. This allows users to re-use and share snippets of code that can be re-used across
many charts, avoiding redundancy and keeping charts [DRY](https://en.wikipedia.org/wiki/Don%27t_repeat_yourself).

Library charts are declared in the dependencies directive in Chart.yaml, and are installed and managed like any other
chart.

```
dependencies:
  - name: mylib
    version: 1.x.x
    repository: quay.io
```

We’re very excited to see the use cases this feature opens up for chart developers, as well as any best practices that
arise from consuming library charts.

### CLI Command Renames

In order to better align the verbiage from other package managers, `helm delete` was re-named to
`helm uninstall`. `helm delete` is still retained as an alias to `helm uninstall`, so either form
can be used.

In Helm 2, in order to purge the release ledger, the `--purge` flag had to be provided. This
functionality is now enabled by default. To retain the previous behaviour, use
`helm uninstall --keep-history`.

Additionally, several other commands were re-named to accommodate the same conventions:

- `helm inspect` -> `helm show`
- `helm fetch` -> `helm pull`

These commands have also retained their older verbs as aliases, so you can continue to use them in either form.

### Automatically creating namespaces

When creating a release in a namespace that does not exist, Helm 2 created the
namespace.  Helm 3 follows the behavior of other Kubernetes objects and returns
an error if the namespace does not exist.

## Installing

### Why aren't there Debian/Fedora/... native packages of Helm?

We'd love to provide these or point you toward a trusted provider. If you're
interested in helping, we'd love it. This is how the Homebrew formula was
started.

### Why do you provide a `curl ...|bash` script?

There is a script in our repository (`scripts/get`) that can be executed as
a `curl ..|bash` script. The transfers are all protected by HTTPS, and the script
does some auditing of the packages it fetches. However, the script has all the
usual dangers of any shell script.

We provide it because it is useful, but we suggest that users carefully read the
script first. What we'd really like, though, are better packaged releases of
Helm.

### How do I put the Helm client files somewhere other than ~/.helm?

Set the `$HELM_HOME` environment variable, and then run `helm init`:

```console
export HELM_HOME=/some/path
helm init --client-only
```

Note that if you have existing repositories, you will need to re-add them
with `helm repo add...`.


## Uninstalling

### I want to delete my local Helm. Where are all its files?

Along with the `helm` binary, Helm stores some files in `$HELM_HOME`, which is
located by default in `~/.helm`.


## Troubleshooting

### On GKE (Google Container Engine) I get "No SSH tunnels currently open"

```
Error: Error forwarding ports: error upgrading connection: No SSH tunnels currently open. Were the targets able to accept an ssh-key for user "gke-[redacted]"?
```

Another variation of the error message is:


```
Unable to connect to the server: x509: certificate signed by unknown authority

```

The issue is that your local Kubernetes config file must have the correct credentials.

When you create a cluster on GKE, it will give you credentials, including SSL
certificates and certificate authorities. These need to be stored in a Kubernetes
config file (Default: `~/.kube/config` so that `kubectl` and `helm` can access
them.

### Why do I get a `unsupported protocol scheme ""` error when trying to pull a chart from my custom repo?**

(Helm < 2.5.0) This is likely caused by you creating your chart repo index without specifying the `--url` flag.
Try recreating your `index.yaml` file with a command like `helm repo index --url http://my-repo/charts .`,
and then re-uploading it to your custom charts repo.

This behavior was changed in Helm 2.5.0.
