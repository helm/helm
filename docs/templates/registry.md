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

For a working example of a template registry, please see the
[Kubernetes Template Registry](https://github.com/kubernetes/application-dm-templates).

## Template Versions

Since templates referenced by configurations and by other templates may change
over time, we need a versioning scheme, so that template references can be reliably
resolved to specific template versions.

Every template must therefore carry a version based on the
[Semantic Versioning](http://semver.org/) specification. A template version
consists of a MAJOR version, a MINOR version and a PATCH version, and can
be represented as a three part string starting with the letter `v` and using
dot delimiters between the parts. For example `v1.1.0`. 

Parts may be omitted from right to left, up to but not include the MAJOR
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
the referenced version. However, it will not automatically substitute a higher
MINOR version for a requested MINOR version with the same MAJOR version, since
although it would be backward compatible, it would not have the same feature set.
You must therefore explicitly request the higher MINOR version in this situation
to obtain the additional features.

## Template Validation

Every template version should include a configuration named `example.yaml`
that can be used to deploy an instance of the template. This file, along with
any supporting files it requires, may be used automatically in the future by
a template testing framework to validate the template, and should therefore be
well formed. 

## Template Organization

Technically, all you need to reference a template is a directory at a public
HTTP endpoint that contains a template file named either `<template-name>.py`
or `<template-name>.jinja`, depending on the implementation language, along 
with any supporting files it might require, such as an optional schema file 
named `<template-name>.py.schema` or `<template-name>.jinja.schema`, respectively, 
helper files used by the implementation, files imported by the schema, and so on.

### Basic structure

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

Also, since it may reduce management overhead to store many different templates,
and/or many versions of the same template, in a single repository, we need a way
to organize templates within a repository.

Therefore:

* Every template version must live in its own directory named for the version.
* The version directory must contain exactly one top-level template file and
supporting files for exactly one template version.
* All of the versions of a given template must live under a directory named for
the template without extensions.

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

In this example, `templateA` is a template directory, and `v1`, `v1.01`, and 
`v1.1` are template version directories that hold the versions of `templateA`.

### Registry based template references

In general, 
[templates references](https://github.com/kubernetes/deployment-manager/blob/master/docs/design/design.md#template-references)
are just URLs to HTTP endpoints. However, because a template registry follows
the conventions outlined above, references to templates in a template registry
can be shorter and simpler than generalized template references.

In a registry based template reference, the scheme part of the URL and the name
of the top level template file are omitted, and the version number is delimited
by a colon. So for example, instead of

```
https://raw.githubusercontent.com/ownerA/repository2/master/templateA/v1/templateA.py
```

you can simply write

```
github.com/ownerA/repository2/templateA:v1
```

The general pattern for a registry based template reference is as follows:

```
github.com/<owner>/<repository>/<collection>/<template>:<version>
```

The `collection` segment, described below, is optional, and may be omitted.

### Grouping templates

Of course, a flat list of templates won't scale, and it's unlikely that any
fixed taxonomy would work for all registries. Template directories may therefore
be grouped in any way that makes sense to the repository maintainers.

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

### Template collections

A side effect of allowing arbitrary grouping is that we don't know how to find
templates when searching or listing the contents of a registry without walking
the directory tree down to the leaves and then backtracking to identify template
directories.

Since walking the repository is not very efficient, we introduce the concept of
collections.

#### Definition

A collection is a directory that contains a flat list of templates. Deployment
manager will only discover templates at the root of a collection.

So for example, `templateA` and `templateB` live in the `templates` collection
in the first example above, and in the `big-data` collection in the second example.

A registry may contain any number of collections. A single, unnamed collection
is implied at the root of every registry, but additional named collections may
be created at other points in the directory structure.

#### Usage

Of course, collections are useless if we can't reference them efficiently. A
registry based template reference may therefore include a collection name. A
collection name is the only path segment allowed between the repository name and
the template name. So, for example, this is a valid template reference:

```
github.com/ownerA/repository2/collectionM/templateA:v1
```

but this is not:

```
github.com/ownerA/repository2/multiple/path/segments/are/not/allowed/templateA:v1
```

Because it may appear in a template reference, a collection name must not contain
URL path separators (i.e., slashes). However, it may contain other delimiters
(e.g., dots). So, for example, this is a valid template reference:

```
github.com/ownerA/repository2/dot.delimited.strings.are.allowed/templateA:v1
```

#### Mapping

Currently, deployment manager maps collection names to top level directory names.
This mapping implies that registries can be at most one level deep. Soon, however,
we plan to introduce a metadata file at the top level that maps collection names
to paths. This feature will allow registries to have arbitrary organizations, by
making it possible to place collections anywhere in the directory tree.

When the metadata file is introduced, the current behavior will be the default.
So, if the metadata file is not found in a given registry, or if a given collection
name is not found in the metadata file, then deployment manager will simply map
it to a top level directory name by default. This approach allows us to define
collections at the top level now, and then move them to new locations later without
breaking existing template references.

## Using Template Registries

### Accessing a template registry

The Deployment Manager client, `dm`, can deploy templates directly from a registry
using the following command:

```
$ dm deploy <template-name>:<version>
```

To resolve the template reference, `dm` looks for a template version directory
with the given version in the template directory with the given template name in
the default template registry.

The default is the [Kubernetes Template Registry](https://github.com/kubernetes/application-dm-templates),
but you can set a different default using the `--registry` flag:

```
$ dm --registry my-org/my-repo/my-collection deploy <template-name>:<version>
```

Alternatively, you can specify a complete template reference using the pattern
described above, like this:

```
$ dm deploy github.com/my-org/my-repo/my-collection/<template-name>:<version>
```

If a template requires properties, you can provide them on the command line:

```
$ dm --properties prop1=value1,prop2=value2 deploy <template-name>:<version>
```

### Changing a template registry

DM relies on Github to provide the tools and processes needed to add, modify or
delete the contents of a template registry. Conventions for changing a template
registry are defined by the registry maintainers, and should be published in the
top level README.md or a file it references, following standard Github practices.

The [Kubernetes Template Registry](https://github.com/kubernetes/application-dm-templates)
follows the [workflow](https://github.com/kubernetes/kubernetes/blob/master/docs/devel/development.md#git-setup)
used by Kubernetes.
