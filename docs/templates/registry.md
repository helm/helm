# Template Registries

DM lets configurations instantiate [templates](../design/design.md#templates)
using both [imports](../design/design.md#template-imports) and
[references](../design/design.md#template-references). 

Because template references can use any public HTTP endpoint, they provide
a way to share templates. While you can store templates anywhere you want and
organize them any way you want, you may not be able to share them effectively
without some organizing principles. This document defines conventions for
template registries that store templates in Github and organize them by name
and by version to make sharing easier.

## Template Versions

Since templates referenced by configurations and by other templates may change
over time, we need a versioning scheme, so that template references can be reliably
resolved to specific template versions.

Every template must therefore carry a version based on the
[Semantic Versioning](http://semver.org/) specification. A template version
consists of a MAJOR version, a MINOR version and a PATCH version, and can
be represented as a three part string starting with the letter `v` and using
dot delimiters between the parts. For example `v1.1.0`. 

Parts may be omitted from left to right, up to but not include the MAJOR
version. All omitted parts default to zero. So, for example:

* `v1.1` is equivalent to `v1.1.0`, and
* `v2` is equivalent to `v2.0.0`

As required by Semantic Versioning:

* The MAJOR version must be incremented for incompatible changes
* The MINOR version must be incremented functionality is added in a backwards-compatible
manner, and
* The PATCH version must be incremented for backwards-compatible bug fixes.

When resolving a template reference, DM will attempt to fetch the template with
the highest available PATCH version that has the same MAJOR and MINOR versions as
the referenced version.

## Template Validation

Every template version should include a configuration named `example.yaml`
that can be used to deploy an instance of the template. This file may be used,
along with any supporting files it requires, to validate the template.

## Template Organization

Technically, all you need to reference a template is a directory at a public
HTTP endpoint that contains a template file named either `<template-name>.py`
or `<template-name>.jinja`, depending on the implementation language, along 
with any supporting files it might require, such as an optional schema file 
named `<template-name>.py.schema` or `<template-name>.jinja.schema`, respectively, 
helper files used by the implementation, files imported by the schema, and so on.

These constraints impose a basic level of organization on the template definition
by ensuring that the template and all of its supporting files at least live in the
same directory, and that the template and schema files follow well-defined naming
conventions.

They do not, however, provide any encapsulation. Without additional constraints,
there is nothing to prevent template publishers from putting multiple templates,
or multiple versions of the same template, in the same directory. While there
might be some benefits in allowing templates to share a directory, such as avoiding
the duplication of helper files, the cost of discovering and maintaining templates
would quickly outweigh them as the number of templates in the directory increased.

Every template version must therefore live in its own directory, and that
directory must contain one and only one top-level template file and supporting
files for one and only template version.

Since it may reduce management overhead to store many different templates,
and/or many versions of the same template, in a single repository, we need a way
to organize templates within a repository.

A template repository must therefore place all of the versions of a given
template in directories named for the template versions under a directory named
for the template.

For example:

```
templateA/
  v1/
    example.yaml
    templateA.py
    templateA.py.schema
  v1.0.1/
    example.yaml
    templateA.py
    templateA.py.schema
  v1.1/
    example.yaml
    templateA.py
    templateA.py.schema
    helper.py
```

The template directories may be organized in any way that makes sense to the
repository maintainers.

For example, this flat list of template directories is valid:

```
templates/
  templateA/
    v1/
    ...
  templateB/
    v2/
    ...
```

This example, where template directories are organized by category, is also valid:

```
templates/
  big-data/  
    templateA/
      v1/
      ...
    templateB/
      v2/
      ...
  signals
    templateC/
      v1/
      ...
    templateD/
      v1.1/
      ...
```

## Template Registries

Github is a convenient place to store and manage templates. A template registry
is a Github repository that conforms to the requirements detailed in this document.

For a working example of a template registry, please see the
[Kubernetes Template Registry](https://github.com/kubernetes/deployment-manager/tree/master/templates).

### Accessing a template registry

The Deployment Manager client, `dm`, can deploy templates directly from a registry
using the following command:

```
$ dm deploy <template-name>:<version>
```

To resolve the template reference, `dm` looks for a template version directory
with the given version in the template directory with the given template name.

By default, it uses the Kubernetes Template Registry. However, you can set a
different default using the `--registry` flag:

```
$ dm --registry my-org/my-repo/my-root-directory deploy <template-name>:<version>
```

Alternatively, you can qualify the template name with the path to the template
directory within the registry, like this:

```
$ dm deploy my-org/my-repo/my-root-directory/<template-name>:<version>
```

Specifying the path to the template directory this way doesn't change the default.

For templates that require properties, you can provide them on the command line:

```
$ dm --properties prop1=value1,prop2=value2 deploy <template-name>:<version>
```

### Changing a template registry

DM relies on Github to provide the tools and processes needed to add, modify or
delete the contents of a template registry. Conventions for changing a template
registry are defined by the registry maintainers, and should be published in the
top level README.md or a file it references, following usual Github practices.

The [Kubernetes Template Registry](https://github.com/kubernetes/deployment-manager/tree/master/templates)
follows the [git setup](https://github.com/kubernetes/kubernetes/blob/master/docs/devel/development.md#git-setup)
used by Kubernetes.
