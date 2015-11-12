# Type Registry

The Deployment Manager client allows you to deploy
[template types](https://github.com/kubernetes/deployment-manager/blob/master/docs/design/design.md#templates)
directly from a Github repository. You can use types from existing registries
or integrate with your own repository.

In order for a Github repository to integrate with Deployment Manager, it must
store Deployment Manager templates in a manner that conforms to the required
**Type Registry** structure detailed in this document.

## File structure
The repository must use the following file structure to store Deployment
Manager template types:

```
<repository-root>/
  types/
    <type-1>/
      <version-1>/
        <type-files>
      <version-2>/
        ...
    <type-2>/
      ...
```

## Versions
Types are versioned based on [Semantic Versioning](http://semver.org/), for
example, *v1.1.0*. A type may have any number of versions.

## Type files
Each type version must contain a top-level Deployment Manager template, named
either `<type-name>.py` or `<type-name>.jinja`, depending on the templating
language used for the type.

A
[template schema](https://github.com/kubernetes/deployment-manager/blob/master/docs/design/design.md#template-schemas)
must also be present, named `<template>.schema` (e.g., `my-template.py.schema`).
Other files may exist as part of the type and imported through the schema,
including sub-templates, data files, or other metadata used by the template.

## Test Configuration
Each type version should include an example YAML configuration called
`example.yaml` to be used for deploying an instance of the type. This is useful
for development purposes.

## Sample Registry
An example of a valid type registry repository looks like:

```
/
  types/
    redis/
      v1/
        example.yaml
        redis.jinja
        redis.jinja.schema
    replicatedservice/
      v3/
        example.yaml
        replicatedservice.py
        replicatedservice.py.schema
```

For a working example of a type registry, please see the
[kubernetes/deployment-manager registry](https://github.com/kubernetes/deployment-manager/tree/master/types).

## Using Types
The Deployment Manager client can deploy types directly from a registry with
the following command:

```
$ dm deploy <type-name>:<version>
```

This will default to the type registry in the kubernetes/deployment-manager
Github repository. You can change this to another repository that contains a
registry with the `--registry` flag:

```
$ dm --registry my-repo/registry deploy <type-name>:<version>
```

For types that require properties:

```
$ dm --properties prop1=value1,prop2=value2 deploy <type-name>:<version>

