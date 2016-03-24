# Helm Charts

This document describes the Helm Chart format, its presentation as an
archive, and its storage and retrieval.


* [Changes](#changes)
* [tl;dr: Summary](#tldr-summary)
* [Goals](#goals)
  * [Non-Goals](#non-goals)
* [The Chart Format](#the-chart-format)
  * [Directory Layout](#directory-layout)
  * [Generators and Templates](#generators-and-templates)
    * [The Generator Header](#the-generator-header)
    * [Python/Jinja Templates (aka DM Templates)](#pythonjinja-templates-aka-dm-templates)
    * [Go Templates (formerly Helm Templates)](#go-templates-formerly-helm-templates)
  * [The Chart File](#the-chart-file)
    * [Dependency Information](#dependency-information)
  * [Releasing a Chart](#releasing-a-chart)
* [The Chart Repository](#the-chart-repository)
  * [The Provenance File](#the-provenance-file)
  * [Repository Protocol](#repository-protocol)
  * [Aside: Why a Flat Repository Namespace?](#aside-why-a-flat-repository-namespace)
  * [Aside: Why Not Git(Hub) or VCS?](#aside-why-not-github-or-vcs)
* [Chart References: Long form, short form, and local reference](#chart-references-long-form-short-form-and-local-reference)
  * [Long Form](#long-form)
  * [Short Form](#short-form)
  * [Local References](#local-references)
* [Appendix A: User Stories for Charts](#appendix-a-user-stories-for-charts)
* [Appendix B: The Standard Chart Repository](#appendix-b-the-standard-chart-repository)
* [Appendix C: EBNF for Generate Header](#appendix-c-ebnf-for-generate-header)
* [Appendix D: References](#appendix-d-references)


## Changes

| Date        | Author        | Changes                                                |
| ------------|:-------------:|:-------------------------------------------------------|
| 2016-01-21  | mbutcher@deis | Added manifests/ to chart.                             |
| 2016-01-25  | mbutcher@deis | Added clarifications based on comments.                |
| 2016-01-26  | mbutcher@deis | Added hook/, removed manifests/. Added generate header.|
| 2016-03-24  | mbutcher@deis | Updated title and intro of document.                   |


## tl;dr: Summary
* A **Chart** is a folder or archive containing a _Chart.yaml_ file, a _templates/_ directory with one or more template files and supporting files, such as schemas and UI directives, and optionally a _README.md_ and _LICENSE_ file.
* When a chart is **released**, it is tested, versioned, provenanced, and loaded into a chart repository.
* A **Chart Repository** is organized according to the conventions of object storage. It is accessible by combining a domain with a repository ID (bucket), and can be browsed. A chart repository contains one or more versioned charts.
* There are three ways to reference charts. A **local reference** references a chart by relative or absolute file system path. It is intended for development. A **long name**, or fully qualified URL, refers to a released chart (in a chart repository) by URL. A **short name**, or mnemonic, is a shortened reference to a chart in a chart repository. A short name can be converted to a **long name** deterministically, and can therefore be used anywhere a long name can be used.

## Goals
The goal of this proposal is to define the following aspects of Helm-DM charts:


* The **format of a Chart**
* The **layout of a Chart repository**, with recommendations about implementations
* The format of a **short name** (mnemonic name) of a chart, as well as its fully qualified **long name**, and conventions for referencing local charts instead of short/long names.


We assume that we are developing a technology that will enjoy widespread use among developers and operators who…


* Are familiar with general Kubernetes design
* Are capable of reading and writing high-level formats like JSON, YAML, and maybe shell scripts
* Have low tolerance for complexity, and are not willing to become domain experts, but…
* Advanced users may be interested in, and willing to, learn advanced features to build much more interesting charts


Based on these, our design goal is to make it **easy to find, use, author, release, and maintain charts**, while still making it possible for advanced users to build sophisticated charts. Furthermore, we have begun the process of **attaching provenance to charts**.


We have attempted to build a proposal that does not lose any of the functionality of DM, and which loses only Helm functionality that we believe is disposable. But both tools will need to change substantial portions of their chart-handling logic.
### Non-Goals
This proposal doesn’t define how either the client or the server must use this data, though we make some non-normative recommendations (e.g. “this data may be displayed to the user” or “a suitable backend will be able to translate this to a fully qualified URL”).


Consequently, this document does not describe the implementation of either client or server sides of this spec. While it defines the pieces necessary for developing and deploying a chart, it does not define a development workflow. Some considerations are listed in [Appendix B](#appendix-b-the-standard-chart-repository).


## The Chart Format
Borrowing the nomenclature from Helm, we define a Chart as a bundle of data and metadata whose contents are sufficient for installing a workload into a Kubernetes cluster running DM.


A Chart is composed of the following pieces:


1. A human- and machine-readable metadata file called `Chart.yaml`
2. A directory named `templates/`
3. An optional `README.md` file
4. An optional `LICENSE` file
5. An optional `docs/` directory
6. An optional `hooks/` directory
7. An optional `icon.svg.`


A chart _may_ have other directories and files, but such files are not defined by or required by this proposal. For the purposes of this proposal, any additional files do not contribute to the core functionality of chart installation.


_Note: When a chart file is deployed, a [provenance file](#the-provenance-file) is generated for the chart. That file is not stored inside of the chart, but is considered part of the chart’s packaged format._


The chart.yaml file format is described [later in this document](#the-chart-file).


The `templates/` directory contains one or more template files, as defined in the [Directory Layout](#directory-layout) section. Templates, along with all of their supporting files, are located therein.


An optional `README.md` file may be specified, which contains long-form text information about using this chart. Tools may display this information, if present. The README.md file is in Markdown format, and should contain information about a Chart’s purpose, usage, and development.


An optional `LICENSE` file may be present, which specifies license information for this chart and/or the images dependent on it.


An optional `docs/` directory may be present, and may contain one or more documentation files. This directory contains documentation that is more specific, verbose, or thorough than what is present in the `README.md` file.


An optional `hooks/` directory may be present. **This is reserved for future use.** A deployment engine may specify lifecycle hooks, such as `pre-install`, `pre-render`, `post-install` and `post-render`. Lifecycle hooks provide an opportunity for a chart author to run additional steps in the processing pipeline. Implementations of these hooks are stored here. _No other files may be stored in this directory._

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


Charts must not assume that they can perform file system operations to load another chart or supporting resources directly via the filesystem, nor should they store any operational files outside of the chart directory. This point becomes important in the case where there is python/jinja or other executable code inside the chart. These executable components especially should not assume they can access the filesystem. It should be possible to archive the chart directory (e.g. `tar -cf ROOT.tar ROOT/`) and not lose any necessary information. A chart is an all-encompassing unit that can be processed by the client/server.

### Generators and Templates
The contents inside of the `templates/` directory may contain directives that can be processed by an external processor. Such a processor may read these files and generate new content.

### The Chart File
The `Chart.yaml` file specifies package metadata for the definition. Package metadata is any information that explains the package structure, purpose, and provenance. Its intended audience is tooling that surfaces such information to the user (e.g. a command line client, a web interface, a search engine).


A definition file does not specify anything about individual pieces of the definition (e.g. description of per-field schema or metadata), nor does it contain information about the parent environment (e.g. that it is hosted on S3 or in GitHub). Its scope is the definition as a whole.


Fields:


* Name: A human-readable name of the definition. This may contain UTF-8 alphanumeric text, together with dash and underscore characters.
* Description: A human-readable description of the definition, not to exceed one paragraph of text. No text formatting is supported. A description may use any printable text characters allowed by the file format.
* Version: A SemVer 2 semantic version of the chart (template files). Refer to the [instruction on semver.org](http://semver.org/).
* Keywords: A list of human-readable keywords. A keyword may contain alphanumeric text and spaces.
* Maintainers: A list of author objects, where an author object has two properties:
   ** Name: Author name
   ** Email: Author email (optional)
* Source: A URL to the source code for this chart
* Home: A URL to the home page for the project
* Dependencies:
   * Charts:
      * Name: definition name
      * Location: Full URL where definition can be found
      * Version: SemVer version range (e.g. ~1.2, `^2`, `>=4.1.1,<5.0.0`). If no version is present, fall back to the Location field (if this has a long name or short name, version info may be present). If there is no version there, use the latest version as found in the destination repository.
* Environment
   * Name: Currently, always Kubernetes.
   * Version: Version of Kubernetes required. SemVer version ranges are supported for this field (e.g. ~1.2).
   * Extensions: List of extension names that are required (`extensions/v1beta1`). These should correspond to paths inside of the `apis/` endpoint on Kubernetes.
   * APIGroups: List of API groups this chart requires in order to function.

```
Example:


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
depends:
        charts:
          - name: memcached
                location: helm.sh/charts/memcached
                version: 0.9.3
        kubernetes:
                version: “>=1.0.0”
                extensions:
                  - extensions/v1beta1
```

#### Dependency Information
The chart format allows the specification of _runtime dependencies_ (not build-time dependencies). For example, a Wordpress chart might have a runtime dependency on a MySQL database. The database needn’t be present for processing the template file, but must be present in the cluster in order for Wordpress to function.


Dependencies hold between charts. That is, one chart may depend upon another chart.


The resolution of dependencies is not covered in this proposal, as it is an implementation detail of the DM service.
### Releasing a Chart
A chart is _released_ when the source of the chart is tested, versioned, packaged into a gzipped tar file, and provenanced. Testing and provenancing attach badges to the Chart that attest to its quality and provenance. At that point, that particular release of a chart is considered immutable. No further changes are allowed. To enforce immutability through tamper detection, Charts must be digitally signed.


Releases must follow a SemVer 2 version pattern.


A released chart may be moved into a chart repository.


NON-NORMATIVE: A release pattern _might_ look like this:

```
$ helm release -r 1.1.2 ./nginx/
-> generated archive
-> signed archive. Signature ‘iEYEARECAAYFAkjil’
-> generated ./nginx-1.1.2.tgz
-> generated ./nginx-1.1.2.prov
-> uploading to https://helm.sh/charts/nginx/1.1.2
-> done
```

## The Chart Repository
A _Chart Repository_ is a place where _released copies_ of one or more charts may reside. (Unreleased charts are explicitly disallowed.) A Helm-DM Chart Repository is analogous to a Debian package repository or a Ruby Gems repository. It is a remote storage location accessible over a well-documented protocol (HTTP(S), and following fixed structural conventions.


Chart repositories are styled after the predominant pattern on [object storage systems](https://cloud.google.com/storage/docs/key-terms). A _domain_ hosts one or more _repositories_. A repository is a bucket in which one or more charts may be stored. In an object storage system, this is represented by the pattern: **https://domain/bucket/object**. In object storage, the _object_ part of the URL may contain slashes, hyphens, underscores, and dots. Thus in the URL [https://storage.googleapis.com/helm/nginx-1.1.2] the object is _nginx-1.1.2_. The general pattern of a chart repository is: https://domain/repository/chart


A chart name should be of the form _name-version.ext_, where _name_ is the chart name (alpha-numeric plus dash and underscore), and version is a SemVer v2 version. The extension, _ext_, should reflect the type of compression used to archive the object. This specification only discusses gzipped tar archives, but other mechanisms could be supported.


Every release should have an accompanying provenance (_.prov_) file. Its name should be exactly the same as the versioned file, but with the prov extension.


Because of the way object storage systems work, a repository should be viewable as a directory tree:

```
https://helm.sh/charts/
        apache-2.3.4.tgz
        apache-2.3.4.prov
        nginx-1.1.1.tgz
        nginx-1.1.1.prov
        nginx-1.1.2.tgz
        nginx-1.1.2.prov
        redis-0.4.0-alpha.1.tgz
        redis-0.4.0-alpha.1.prov

```

A helm chart is a gzip-compressed tar archive (e.g. `tar -zcf …`). Each chart has an accompanying signature that provides a signed representation of the Chart.yaml file. This is for cryptographic verification and provenance information.

### The Provenance File
The provenance file contains a chart’s YAML file plus several pieces of verification information. Provenance files are designed to be automatically generated.


The following pieces of provenance data are added:


* The chart file (Chart.yaml) is included to give both humans and tools an easy view into the contents of the chart.
* Every image file that the project references is checksummed (SHA-256?), and the sum included here. If two versions of the same image are used by the template, both checksums are included.
* The signature (SHA-256) of the chart package (the .tgz file) is included, and may be used to verify the integrity of the chart package.
* The entire body is signed using PGP (see [http://keybase.io] for an emerging way of making crypto signing and verification easy).


The combination of this gives users the following assurances:


* The images this chart references at build time are still the same exact version when installed (checksum images).
   * This is distinct from asserting that the image Kubernetes is running is exactly the same version that a chart references. Kubernetes does not currently give us a way of verifying this.
* The package itself has not been tampered with (checksum package tgz).
* The entity who released this package is known (via the GPG/PGP signature).


The format of the file is as follows:

```
-----BEGIN PGP SIGNED MESSAGE-----
name: nginx
description: The nginx web server as a replication controller and service pair.
version: 0.5.1
keywords:
  - https
  - http
  - web server
  - proxy
source: https://github.com/foo/bar
home: http://nginx.com
depends:
        kubernetes:
                version: >= 1.0.0
---
files:
        nginx-0.5.1.tgz: “sha256:9f5270f50fc842cfcb717f817e95178f”
images:
        “hub.docker.com/_/nginx:5.6.0”: “sha256:f732c04f585170ed3bc99”
-----BEGIN PGP SIGNATURE-----
Version: GnuPG v1.4.9 (GNU/Linux)


iEYEARECAAYFAkjilUEACgQkB01zfu119ZnHuQCdGCcg2YxF3XFscJLS4lzHlvte
WkQAmQGHuuoLEJuKhRNo+Wy7mhE7u1YG
=eifq
-----END PGP SIGNATURE-----
```

Note that the YAML section contains two documents (separated by ---\n). The first is the Chart.yaml. The second is the checksums, defined as follows.


* Files: A map of filenames to SHA-256 checksums (value shown is fake/truncated)
* Images: A map of image URLs to checksums (value shown is fake/truncated)


The signature block is a standard PGP signature, which provides [tamper resistance](http://www.rossde.com/PGP/pgp_signatures.html).

### Repository Protocol
A repository must implement HTTP(S) GET operations on both chart names (nginx-1.2.3) and chart signatures (nginx-1.2.3.sig). HTTP GET is a rational base level of functionality because it is well understood, nearly ubiquitous, and simple.


A repository may implement the full Object Storage APIs for compatibility with S3 and Google Cloud Storage. In this case, a client may detect this and use those APIs to perform create, read, update, delete, and list operations (as permissions allow) on the object storage system.


Object storage is a rational choice for this model because it is [optimized for highly resilient, distributed, highly available read-heavy traffic](https://en.wikipedia.org/wiki/Object_storage). The S3-like API has become a de facto standard, with myriad client libraries and tools, and major implementations both as services (S3, GCS), platforms (OpenStack Swift), and individual stand-alone servers (RiakCS, Minio, Ceph).


This proposal does not mandate a particular authentication mechanism, but implementors may implement the authentication token mechanisms on GCS and S3 compatible object storage systems.

### Aside: Why a Flat Repository Namespace?
The format for a repository has packages stored in a flat namespace, with version information appended to the filename. There are a few reasons we opted for this over against more hierarchical directory structures:


* Keeping package name and version in the file name makes the experience easier for development. In cases where manual examination is necessary, tools and humans are working with filenames that are easily identifiable (_nginx-1.2.3.tgz_).
* The flat namespace means tooling needs to pay less attention to directory hierarchy, which has three positive implications:
   * Less directory recursion, especially important for base operations like counting charts in a repo.
   * No importance is granted to directories, which means we could introduce non-semantic directory hierarchy if necessary (e.g. for the GitHub model of _username/reponame_.
   * For object storage systems, where slashes are part of names but with special meaning, the flat namespace reduces the number of API operations that need to be done, and allows for using some of the [object storage URL components](https://cloud.google.com/storage/docs/json_api/v1/) when necessary.
* Brevity is desirable when developers have occasion to type these URLs directly, as is done in chart/template references (see, for example, the [S3 API](http://docs.aws.amazon.com/AmazonS3/latest/dev/UsingObjects.html)).

### Aside: Why Not Git(Hub) or VCS?
In their first incarnations, both Helm and DM used GitHub-backed repositories for released charts. The Helm team believes we made a huge mistake by going this route. This method suffers from some severe limitations (listed below). The proposal above is designed to work around those limitations.


* Versioning Multiple Things
   * Git does not provide a good way of versioning independent objects in the same repo. (Two attempts we made at solving this [here](https://github.com/helm/helm/issues/199) and [here](https://github.com/deis/helm-dm/issues/1))
   * A release should be immutable, or very close to it. Achieving this in Git is hard if the thing released is not the entire repository.
* Developer Experience
   * Combining release artifacts and source code causes confusion
   * Substantial development workflow overhead (and over-reliance on conventions) make directory-based versioning problematic
   * In most of the models specified, there is no way of determining whether a resource is the in development, or is complete
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
The [Google Storage scheme](https://cloud.google.com/storage/docs/interoperability) (`gs://`)  and the S3 scheme (`s3://`) can also be support. These forms do not follow the URL standard (bucket name is placed in the host field), but their behavior is well-documented.


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

For the purpose of providing versioning ranges, and also for backward compatibility, version requirements may be treated as as suffix condition, instead of part of the path:

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



## Appendix A: User Stories for Charts
Personas:


- Operator: Responsible for running an application in production.
- Service Dev: Responsible for developing new definitions
- App Dev: Developer who creates applications that make use of existing definitions, but does not create definitions.


Stories:


- As an operator, I want a build that is 100% reproducible (exact versions)
- As an app dev, I want to be able to search for chart definitions using keys definied in the [chart file](#the-chart-file)...
        * by keyword, where one app may have multiple keywords (e.g. Redis has storage, message queue)
        * by name (meaning name of the chart), where name may be "fuzzy".
        * by author
        * by last updated date
- As a service dev, I want a well-defined set of practices to follow
- As a service dev, I want to be able to work with a team on the same definition
- As a service dev, I want to be able to indicate when a particular definition is stable, and how stable it is
- As a service dev, I want to indicate the role I played in building a definition
- As a service dev, I want to be able to use all of the low-level Kubernetes kinds
- As an operator, I want to be able to determine how stable a package is
- As an operator, I want to be able to determine what version of Kubernetes I need to run a definition
- As an operator, I want to determine whether a definition requires extension kinds (e.g. DaemonSet or something custom), and determine this _before_ I try to install
- As a service dev, I want to be able to express that my definition depends on others, even to the extent that I specify the version or version range of the other definition upon which I depend
- As a service dev, I do not want to install additional tooling to write, test, or locally run a definition (this relates to the file format in that the format should not require additional tooling)
- As a service dev, I want to be able to store auxiliary files of arbitrary type inside of a definition (e.g. a PDF with documentation, or a logo for my product)
- As a service dev, I want to be able to store my definition in one repository, but reference a definition in another repository
- As a service dev, I want to embed my template inside of the code that it references. For example, I want to have the code to build a docker image and the definition to all live in the same source code repository.


## Appendix B: The Standard Chart Repository
We have talked about adding a standard (default) chart repository that we are the gatekeepers on. This would hold only curated/vetted charts over which we have control. The following table illustrates how we’re thinking about implementing this:

    +-----------+----------------+----------------+-------------+--------------+-----------+
    |Stage      |Devel           |Review          |Release      |Store         |Use        |
    +-----------+----------------+----------------+-------------+--------------+-----------+
    |Operations | Create chart   |Code review     |Sign         |Store releases|Get        |
    |           | Modify chart   |Test            |Version      |              |Unpack     |
    |           |                |                |Package      |              |Deploy     |
    +-----------+----------------+----------------+-------------+--------------+-----------+
    |Who?       | Developer      | Us             | Us          | Us           | Anyone    |
    +-----------+----------------+----------------+-------------+--------------+-----------+
    |How?       |Dev creates     |We review the   |We issue a   |We load into  |The chart  |
    |           |chart & issues  |pull request    |Helm release,|our repository|is then    |
    |           |issues a pull   |                |signing as   |              |publicly   |
    |           |request         |                |we go        |              |available  |
    +-----------+----------------+----------------+-------------+--------------+-----------+

One possible implementation of this would be to have a single GitHub repository of charts. The workflow would look like this:


1. Developer forks the repo
2. Developer submits their changes (new chart, modified chart) as a pull request
3. Automatic linter vets the chart immediately
4. We review, and iterate with the developer on any changes
5. We release (sign and package, and then push to object storage)
6. Chart is then publicly available


An internal workflow for a company who wants to host their own repository, in contrast, would look like this:

    +-----------+----------------+----------------+-------------+---------------+-----------+
    |Stage      |Devel           |Review          |Release      |Store          |Use        |
    +-----------+----------------+----------------+-------------+---------------+-----------+
    |Operations |Create chart    |Code review     |Sign         |Store releases |Get        |
    |           |Modify chart    |Test            |Version      |               |Unpack     |
    |           |                |                |Package      |               |Deploy     |
    +-----------+----------------+----------------+-------------+---------------+-----------+
    |Who?       | Them           | Them           | Them        | Them          | Them      |
    +-----------+----------------+----------------+-------------+---------------+-----------+
    |How?       |They create     |They do their   |They aquire  |They create    |They have  |
    |           |their own       |own CI/CD       |their own    |their own obj  |the ability|
    |           |source control  |                |keys and sign|storage repo   |to release |
    |           |workflow        |                |             |and host their |whatever   |
    |           |                |                |             |               |audience   |
    |           |                |                |             |               |they choose|
    +-----------+----------------+----------------+-------------+---------------+-----------+

## Appendix C: EBNF for Generate Header

This is supplied to remove any vagaries of the [examples given](#the-generator-header).

```
commentStart = “#” | “//” | “/*”
commentEnd = “*/”
terminate = “\n” | commentEnd
generate = “helm:generate”
handler = alphaNum
arg = {whitespace, alphaNum}
helmHeader = commentStart, generate, handler, [arg], terminate
alphaNum = ? ASCII a-z A-Z 0-9 plus - and _?
whitespace = {“ “, “\t”}
```

## Appendix D: References

The Debian Package Repo. [http://ftp.us.debian.org/debian/pool/main/h/]

The Debian Maintainers Guide. [https://www.debian.org/doc/manuals/maint-guide/]

Arch packages: [https://wiki.archlinux.org/index.php/Arch_User_Repository#Creating_a_new_package]

Keybase.io: [https://keybase.io/]

Google Cloud Storage API: [https://cloud.google.com/storage/docs/json_api/v1/]

Amazon S3: [http://docs.aws.amazon.com/AmazonS3/latest/dev/Welcome.html]

URIs (RFC 3986): [https://tools.ietf.org/html/rfc3986#section-3.1]
