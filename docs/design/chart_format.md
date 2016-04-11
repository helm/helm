# Helm Charts

This document describes the Helm chart format, its presentation as an
archive, and its storage and retrieval.


* [Changes](#changes)
* [tl;dr: Summary](#tldr-summary)
* [Goals](#goals)
  * [Non-Goals](#non-goals)
* [The Chart Format](#the-chart-format)
  * [Directory Layout](#directory-layout)
  * [The Chart File](#the-chart-file)
  * [Releasing a Chart](#releasing-a-chart)
* [The Chart Repository](#the-chart-repository)
  * [Repository Protocol](#repository-protocol)
  * [Aside: Why a Flat Repository Namespace?](#aside-why-a-flat-repository-namespace)
  * [Aside: Why Not Git(Hub) or VCS?](#aside-why-not-github-or-vcs)
* [Chart References: Long form, short form, and local reference](#chart-references-long-form-short-form-and-local-reference)
  * [Long Form](#long-form)
  * [Short Form](#short-form)
  * [Local References](#local-references)
* [References](#references)


## Changes

| Date        | Author         | Changes                                                |
| ------------|:--------------:|:-------------------------------------------------------|
| 2016-01-21  | mbutcher@deis  | Added manifests/ to chart.                             |
| 2016-01-25  | mbutcher@deis  | Added clarifications based on comments.                |
| 2016-01-26  | mbutcher@deis  | Added hook/, removed manifests/. Added generate header.|
| 2016-03-24  | mbutcher@deis  | Updated title and intro of document.                   |
| 2016-04-01  | dcunnin@google | Updated expander / schema post-DM merge                |


## tl;dr: Summary
* A **chart** is a folder or archive containing a _Chart.yaml_ file, a _templates/_ directory with one or more template files, supporting files, such as schemas and UI directives, and optionally a _README.md_ and _LICENSE_ file.
* When a chart is **released**, it is tested, versioned, and loaded into a chart repository.
* A **chart repository** is organized according to the conventions of object storage. It is accessible by combining a domain with a repository ID (bucket), and can be browsed. A chart repository contains one or more versioned charts.
* There are three ways to reference charts. A **local reference** references a chart by relative or absolute file system path. It is intended for development. A **long name**, or fully qualified URL, refers to a released chart (in a chart repository) by URL. A **short name**, or mnemonic, is a shortened reference to a chart in a chart repository. A short name can be converted to a **long name** deterministically, and can therefore be used anywhere a long name can be used.

## Goals
The goal of this document is to define the following aspects of Helm charts:


* The **format of a chart**
* The **layout of a chart repository**, with recommendations about implementations
* The format of a **short name** (mnemonic name) of a chart, as well as its fully qualified **long name**, and conventions for referencing local charts instead of short/long names.


We assume that we are developing a technology that will enjoy widespread use among developers and operators who…


* Are familiar with general Kubernetes design
* Are capable of reading and writing high-level formats like JSON, YAML, and maybe shell scripts
* Have low tolerance for complexity, and are not willing to become domain experts, but…
* Advanced users may be interested in, and willing to, learn advanced features to build much more interesting charts


Based on these, our design goal is to make it **easy to find, use, author, release, and maintain charts**, while still making it possible for advanced users to build sophisticated charts.

This design is based on the integration of Deployment Manager (DM) and Helm, formerly at github.com/deis/helm. It does not lose any of the functionality of DM, and loses only Helm functionality that we believe is disposable. Substantial portions of the chart and template handling logic from the original implementations of these two tools have changed as a result of this integration. The new chart format, described in this document, is one of the primary drivers of those changes.


### Non-Goals
This document does not define how either the client or the server must use this data, though we make some non-normative recommendations (e.g. “this data may be displayed to the user” or “a suitable backend will be able to translate this to a fully qualified URL”).

Consequently, this document does not describe the implementation of either client or server sides of this spec. While it defines the pieces necessary for developing and deploying a chart, it does not define a development workflow. Development workflows are discussed in [another document](../workflow/team-workflows.md).


## The Chart Format
We define a Chart as a bundle of data and metadata whose contents are sufficient for installing a workload into a Kubernetes cluster.


A Chart is composed of the following pieces:


1. A human- and machine-readable metadata file called `Chart.yaml`
2. A directory named `templates/`
3. An optional `README.md` file
4. An optional `LICENSE` file
5. An optional `docs/` directory
7. An optional `icon.svg`


A chart _may_ have other directories and files, but such files are not defined by or required by this document. For the purposes of this document, any additional files do not contribute to the core functionality of chart installation.


The Chart.yaml file format is described [later in this document](#the-chart-file).


The `templates/` directory contains one or more template files, as defined in the [Directory Layout](#directory-layout) section. Templates, along with all of their supporting files, are located therein.


An optional `README.md` file may be specified, which contains long-form text information about using this chart. Tools may display this information, if present. The README.md file is in Markdown format, and should contain information about a Chart’s purpose, usage, and development.


An optional `LICENSE` file may be present, which specifies license information for this chart and/or the images dependent on it.


An optional `docs/` directory may be present, and may contain one or more documentation files. This directory contains documentation that is more specific, verbose, or thorough than what is present in the `README.md` file.

### Directory Layout
A chart is laid out as follows. The top level directory (represented by the placeholder ROOT) must be the name of the chart (verified by linter). For example, if the chart is named `nginx`, the ROOT directory must be named `nginx/`.

```
ROOT/
    Chart.yaml
    README.md
    LICENSE
    docs/
        some.md
    templates/
        some.yaml
        some.jinja
        some.jinja.schema
```

Templates are stored in a separate directory for the following reasons:


* This future-proofs the format: we can add other directories at the top level and deprecate `templates/`.
* It allows authors to add other files (such as documentation) in a straightforward way that will not confuse definitions-aware tools.
* It allows for the possibility that a chart definition may be embedded inside of project code.


Charts must not assume that they can perform file system operations to load another chart or supporting resources directly via the filesystem, nor should they store any operational files outside of the chart directory. This point becomes important in the case where there is Python/Jinja or other executable code inside the chart. These executable components especially should not assume they can access the host filesystem. It should be possible to archive the chart directory (e.g. `tar -cf ROOT.tar ROOT/`) and not lose any necessary information. A chart is an all-encompassing unit that can be processed by the client/server.

### The Chart File
The `Chart.yaml` file specifies package metadata for the definition. Package metadata is any information that explains the package structure, purpose, and provenance. Its intended audience is tooling that surfaces such information to the user (e.g. a command line client, a web interface, a search engine).


A definition file does not specify anything about individual pieces of the definition (e.g. description of per-field schema or metadata), nor does it contain information about the parent environment (e.g. that it is hosted on S3 or in GitHub). Its scope is the definition as a whole.


Fields:


* name: A human-readable name of the definition. This may contain UTF-8 alphanumeric text, together with dash and underscore characters.
* description: A human-readable description of the definition, not to exceed one paragraph of text. No text formatting is supported. A description may use any printable text characters allowed by the file format.
* version: A SemVer 2 semantic version of the chart (template files). Refer to the [instruction on semver.org](http://semver.org/).
* keywords: A list of human-readable keywords. A keyword may contain alphanumeric text and spaces.
* maintainers: A list of author objects, where an author object has two properties:
   * name: Author name
   * email: Author email (optional)
* source: A URL to the source code for this chart
* home: A URL to the home page for the project
* expander: Indicates how to process the contents of templates/ (optional)
   * name: The name of the expander, as a Kubernetes service name or URL.
   * entrypoint: If the expander requires an entrypoint, gives the file (optional).
* schema: The file used to validate the properties (user-configurable inputs) of this chart before expansion.  (optional)

Example:

```
name: nginx
description: The nginx web server as a replication controller and service pair.
version: 0.5.1
keywords:
* https
* http
* web server
* proxy
source: https://github.com/foo/bar
home: http://nginx.com
expander:
  name: goexpander-service
schema: Schema.yaml
```

### Expanders and Templates
The content of the `templates/` directory and the schema file are defined by the particular expander invoked by name in the Chart.yaml file.  Such expanders consume these files, in the context of properties, to generate content.  If a schema is given, the expander may use it to validate the properties before expansion.  Discussion of the available expanders and how they intepret the content of /templates is outside the scope of this document.

If no expander is specified, files with yaml or json extensions in the templates/ directory are parsed as Kubernetes API objects and included in the deployment without transformation.  Charts may therefore contain Kubernetes API objects that do not contain any parameters or require any server side processing before being sent to Kubernetes.  Such charts can therefore not invoke other charts.

### Releasing a Chart
A chart is _released_ when the source of the chart is tested, versioned, packaged into a gzipped tar file. At that point, that particular release of a chart is considered immutable. No further changes are allowed. To enforce immutability through tamper detection, Charts must be digitally signed.


Releases must follow a SemVer 2 version pattern.


A released chart may be moved into a chart repository.


NON-NORMATIVE: A release pattern _might_ look like this:

```
$ helm release -r 1.1.2 ./nginx/
-> generated archive
-> signed archive. Signature ‘iEYEARECAAYFAkjil’
-> generated ./nginx-1.1.2.tgz
-> uploading to gs://kubernetes-charts/nginx-1.1.2.tgz
-> done
```

## The Chart Repository
A _Chart Repository_ is a place where _released copies_ of one or more charts may reside. A Helm Chart Repository is analogous to a Debian package repository or a Ruby Gems repository. It is a remote storage location accessible over a well-documented protocol (HTTP(S), and following fixed structural conventions.


Chart repositories are styled after the predominant pattern of [object storage systems](https://cloud.google.com/storage/docs/key-terms). A _domain_ hosts one or more _repositories_. A repository is a bucket in which one or more charts may be stored. In an object storage system, this is represented by the pattern: **https://domain/bucket/object**. In object storage, the _object_ part of the URL may contain slashes, hyphens, underscores, and dots. Thus in the URL [https://storage.googleapis.com/helm/nginx-1.1.2] the object is _nginx-1.1.2_. The general pattern of a chart repository is: https://domain/repository/chart


A chart name should be of the form _name-version.ext_, where _name_ is the chart name (alpha-numeric plus dash and underscore), and version is a SemVer v2 version. The extension, _ext_, should reflect the type of compression used to archive the object. This specification only discusses gzipped tar archives, but other mechanisms could be supported.


Because of the way object storage systems work, a repository should be viewable as a directory tree:

```
gs://kubernetes-charts/charts/
        apache-2.3.4.tgz
        nginx-1.1.1.tgz
        nginx-1.1.2.tgz
        redis-0.4.0-alpha.1.tgz
```

A helm chart is a gzip-compressed tar archive (e.g. `tar -zcf …`).

### Repository Protocol
A repository must implement HTTP(S) GET operations on both chart names (nginx-1.2.3) and chart signatures (nginx-1.2.3.sig). HTTP GET is a rational base level of functionality because it is well understood, nearly ubiquitous, and simple.


A repository may implement the full Object Storage APIs for compatibility with S3 and Google Cloud Storage. In this case, a client may detect this and use those APIs to perform create, read, update, delete, and list operations (as permissions allow) on the object storage system.


Object storage is a rational choice for this model because it is [optimized for highly resilient, distributed, highly available read-heavy traffic](https://en.wikipedia.org/wiki/Object_storage). The S3-like API has become a de facto standard, with myriad client libraries and tools, and major implementations both as services (S3, GCS), platforms (OpenStack Swift), and individual stand-alone servers (RiakCS, Minio, Ceph).


This document does not mandate a particular authentication mechanism, but implementors may implement the authentication token mechanisms on GCS and S3 compatible object storage systems.

### Aside: Why a Flat Repository Namespace?
The format for a repository has packages stored in a flat namespace, with version information appended to the filename. There are a few reasons we opted for this over against more hierarchical directory structures:


* Keeping package name and version in the file name makes the experience easier for development. In cases where manual examination is necessary, tools and humans are working with filenames that are easily identifiable (_nginx-1.2.3.tgz_).
* The flat namespace means tooling needs to pay less attention to directory hierarchy, which has three positive implications:
   * Less directory recursion, especially important for base operations like counting charts in a repo.
   * No importance is granted to directories, which means we could introduce non-semantic directory hierarchy if necessary (e.g. for the GitHub model of _username/reponame_.
   * For object storage systems, where slashes are part of names but with special meaning, the flat namespace reduces the number of API operations that need to be done, and allows for using some of the [object storage URL components](https://cloud.google.com/storage/docs/json_api/v1/) when necessary.
* Brevity is desirable when developers have occasion to type these URLs directly, as is done in chart/template references (see, for example, the [S3 API](http://docs.aws.amazon.com/AmazonS3/latest/dev/UsingObjects.html)).

### Aside: Why Not Git(Hub) or VCS?
GitHub-backed repositories for released charts suffer from some severe limitations (listed below). Our repository design does not have these limitations.


* Versioning Multiple Things
   * Git does not provide a good way of versioning independent objects in the same repo. (Two attempts we made at solving this [here](https://github.com/helm/helm/issues/199) and [here](https://github.com/deis/helm-dm/issues/1))
   * A release should be immutable, or very close to it. Achieving this in Git is hard if the thing released is not the entire repository.
* Developer Experience
   * Combining release artifacts and source code causes confusion
   * Substantial development workflow overhead (and over-reliance on conventions) make directory-based versioning problematic
   * In most of the models specified, there is no way of determining whether a resource is in development, or is complete
* Infrastructure
   * Both teams have hit GitHub API rate limiting
   * Git is optimized for fetching an entire repository, not a small fraction of the repository
   * Git is optimized for fetching an entire history, not a snapshot of a particular time
   * Discovery of which versions are available is exogenous to Git itself (unless we use a crazy tagging mechanism)
   * We hit vendor lock-in for GitHub


The object storage based repository solution above solves all of these problems by:


* Explicitly defining a release artifact as an archive file with a known format and a known naming convention.
* Explicitly defining a release as an act of archiving, naming, and signing.
* Selecting a service (object storage, but with fallback to plain HTTP) that is resilient, widely deployed, and built specifically for the delivery of release files.
* Distinguishing the development workflow from the release workflow at a location that is intuitive and common for developers
* Providing a method for verifying immutability of a version by checksum
* Removing vendor reliance
* Separating the concept of code from the concept of release artifact


We want to make clear that a chart may be developed in any place (such as Github repositories). This aside is referring to released and versioned charts that can be stored and shared. We do not dictate where a chart is developed.

## Chart References: Long form, short form, and local reference
There are three reference forms for a chart. A fully qualified (long) form, a mnemonic (short) form, and a local path spec.
### Long Form
A long form chart reference is an exact URL to the chart. It should use HTTP or HTTPS protocols, and follow the typical rules of a complete URL: https://example.com/charts/nginx-1.2.3.tgz


A long form reference must include a protocol spec. It may also contain any pattern allowed in URLs.


The following URI schemes must be supported:
* http:
* https:


#### Special Form Google Storage and S3
The [Google Storage scheme](https://cloud.google.com/storage/docs/interoperability) (`gs://`)  and the S3 scheme (`s3://`) can also be supported. These forms do not follow the URL standard (bucket name is placed in the host field), but their behavior is well-documented.


Examples:
```
gs://charts/nginx-1.2.3.tgz
s3://charts/nginx-1.2.3.tgz
```
### Short Form
A short form package reference provides the minimum necessary information for accessing a chart. Short form URLs use a defined [URI scheme](https://tools.ietf.org/html/rfc3986#section-3.1) of `helm`:.


A generic short form maps to an object storage reference (`DOMAIN/REPOSITORY/RELEASE`).

```
helm:example.com/charts/nginx-1.2.3.tgz
```

Or, in the case of the Google Storage (`gs://`) and the S3 scheme (`s3://`), the domain indicates the storage scheme.

```
helm:gs/kubernetes-charts/nginx-1.2.3.tgz
```

For the purpose of providing versioning ranges, and also for backward compatibility, version requirements are expressed as a suffix condition, instead of as part of the path:

```
helm:example.com/charts/nginx#1.2.3   // Exact version
helm:example.com/charts/nginx#~2.1    // Version range
helm:example.com/charts/nginx         // Latest release
```

The first of the above three short names is equivalent to helm:example.com/charts/nginx-1.2.3.tgz. The second example uses a semantic version filter ~1.2, which means “>=1.2.0, <1.3.0”, or “any patch version of 1.2”. Other filters include the “^” operator (^1 is “any minor version in the 1.x line), and “>”, “>=”, “=”, “<”, and “<=” operators. These are standard SemVer filter operators.


Any short form handler should be able to resolve the default short form as specified above.


### Local References
During chart development, and in other special circumstances, it may be desirable to work with an unversioned, unpackaged local copy of a chart. For the sake of consistency across products, an explicit naming formula is to be followed.


In cases where a local path is used in lieu of a full or short name, the path string _must_ begin with either a dot (.) or a slash (/) or the file schema(`file:`).


Legal examples of this include:
```
  ./foo
  ../foo
  /foo
  //foo
  /.foo
  file:///example/foo
```

Unprefixed relative paths are not valid. For example, `foo/` is not allowed as a local path, as it conflicts with a legal short name, and is thus ambiguous.


## References

The Debian Package Repo. [http://ftp.us.debian.org/debian/pool/main/h/]

The Debian Maintainers Guide. [https://www.debian.org/doc/manuals/maint-guide/]

Arch packages: [https://wiki.archlinux.org/index.php/Arch_User_Repository#Creating_a_new_package]

Keybase.io: [https://keybase.io/]

Google Cloud Storage API: [https://cloud.google.com/storage/docs/json_api/v1/]

Amazon S3: [http://docs.aws.amazon.com/AmazonS3/latest/dev/Welcome.html]

URIs (RFC 3986): [https://tools.ietf.org/html/rfc3986#section-3.1]
