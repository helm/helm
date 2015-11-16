# Deployment Manager Design

## Overview

Deployment Manager (DM) is a service that runs in a Kubernetes cluster,
supported by a command line interface. It provides a declarative `YAML`-based
language for configuring Kubernetes resources, and a mechanism for deploying,
updating, and deleting configurations. This document describes the configuration
language, the API model, and the service architecture in detail.

## Configuration Language

DM uses a `YAML`-based configuration language with a templating mechanism. A
configuration is a `YAML` file that describes a list of resources. A resource has
three properties:

* `name`: the name to use when managing the resource
* `type`: the type of the resource being configured
* `properties`: the configuration properties of the resource

Here's a snippet from a typical configuration file:

```
resources:
- name: my-rc
  type: ReplicationController
  properties:
    metadata:
      name: my-rc
    spec:
      replicas: 1
    ...
- name: my-service
  type: Service
  properties:
    ...
```

It describes two resources:

* A replication controller named `my-rc`, and 
* A service named `my-service`

## Types

Resource types are either primitives or templates.

### Primitives

Primitives are types implemented by the Kubernetes runtime, such as:

* `Pod`
* `ReplicationController`
* `Service`
* `Namespace`
* `Secret`

DM processes primitive resources by passing their properties directly to
`kubectl` to create, update, or delete the corresponding objects in the cluster.

(Note that DM runs `kubectl` server side, in a container.)

### Templates

Templates are abstract types created using Python or
[Jinja](http://jinja.pocoo.org/). A template takes a set of properties as input,
and must output a valid `YAML` configuration. Properties are bound to values when
a template is instantiated by a configuration.

Templates are expanded before primitive resources are processed. The 
configuration produced by expanding a template may contain primitive resources
and/or additional template invocations. All template invocations are expanded
recursively until the resulting configuration is a list of primitive resources.

(Note, however, that DM preserves the template hierarchy and any dependencies
between resources in a layout that can be used to reason programmatically about
the structure of the resulting collection of resources created in the cluster,
as described in greater detail below.)

Here's an example of a template written in [Jinja](http://jinja.pocoo.org/):

```
resources:
- name: {{ env['name'] }}-service
  type: Service
  properties:
    prop1: {{ properties['prop1'] }}
    ...
```

As you can see, it's just a `YAML` file that contains expansion directives. For
more information about the kinds of things you can do in a Jinja based template,
see [the Jina documentation](http://jinja.pocoo.org/docs/).

Here's an example of a template written in Python:

```
import yaml

def GenerateConfig(context):
  resources = [{
    'name': context.env['name'] + '-service',
    'type': 'Service',
    'properties': {
      'prop1': context.properties['prop1'],
      ...
    }
  }]

  return yaml.dump({'resources': resources})
```

Of course, you can do a lot more in Python than in Jinja, but basic things, such
as simple parameter substitution, may be easier to implement and easier to read in
Jinja than in Python.

Templates provide access to multiple sets of data that can be used to 
parameterize or further customize configurations:

* `env`: a map of key/value pairs from the environment, including pairs
defined by Deployment Manager, such as `deployment`, `name`, and `type`
* `properties`: a map of the key/value pairs passed in the properties section
of the template invocation
* `imports`: a map of import file names to file contents for all imports
originally specified for the configuration

In Jinja, these variables are available in the global scope. In Python, they are
available as properties of the `context` object passed into the `GenerateConfig`
method.

### Template schemas

A template can optionally be accompanied by a schema that describes it in more
detail, including:

* `info`: more information about the template, including long description and title
* `imports`: any files imported by this template (may be relative paths or URLs)
* `required`: properties that must have values when the template is expanded
* `properties`: A `JSON Schema` description of each property the template accepts

Here's an example of a template schema:

```
info:
  title: The Example
  description: A template being used as an example to illustrate concepts.

imports:
- path: helper.py

required:
- prop1

properties:
  prop1:
    description: The first property
    type: string
    default: prop-value
```

When a schema is provided for a template, DM uses it to validate properties 
passed to the template by its invocation, and to provide default values for
properties that were not given values.

Schemas must be supplied to DM along with the templates they describe.

### Supplying templates

Templates can be supplied to DM in two different ways:

* They can be passed to DM along with configurations that import them, or 
* They can be retrieved by DM from public HTTP endpoints for configurations that
reference them.

#### Template imports

Configurations can import templates using path declarations. For example:

```
imports:
- path: example.py

resources:
- name: example
  type: example.py
  properties:
    prop1: prop-value
```

The `imports` list is not understood by the Deployment Manager service.
It's a directive used by client-side tools to specify what additional files
should be included when passing the configuration to the API.

If you are calling the Deployment Manager service directly, you must embed the
imported templates in the configuration passed to the API.

#### Template references

Configurations can also reference templates using URLs for public HTTP endpoints.
DM will attempt to resolve template references during expansion. For example:

```
resources:
- name: my-template
  type: https://raw.githubusercontent.com/my-template/my-template.py
  properties:
    prop1: prop-value
```

When resolving template references, DM assumes that templates are stored in
directories, which may also contain schemas, examples and other supporting files.
It therefore processes template references as follows:

1. Attempt to fetch the template, and treat it as an import.
1. Attempt to fetch the schema for the template from
`<base path>/<template name>.schema`
1. Attempt to fetch files imported by the schema from `<base path>/<import path>`

Referring to the previous example, 

* the base path is `https://raw.githubusercontent.com/my-template`, 
* the template name is `my-template`, and
* the schema name is `my-template.schema`

If we include a configuration that uses the template as an example, then the
directory that contains `my-template` might look like this:

```
example.yaml
my-template.py
my-template.py.schema
helper.py
```

### Value references
Resources can reference values from other resources. The version of Deployment
Manager running in the Google Cloud Platform uses references to understand
dependencies between resources and properly order the operations it performs on
a configuration.

(Note that this version of DM doesn't yet order operations to satisfy 
dependencies, but it will soon.)

A reference follows this syntax: `$(ref.NAME.PATH)`, where `NAME` is the name
of the resource being referenced, and `PATH` is a `JSON` path to the value in the
resource object.

For example:

```
$(ref.my-service.metadata.name)
```

In this case, `my-service` is the name of the resource, and `metadata.name` is
the `JSON` path to the value being referenced.

## API Model

DM exposes a set of RESTful collections over HTTP/JSON.

### Deployments

Deployments are the primary resources managed by the Deployment Manager service.
The inputs to a deployment are:

* `name`: the name by which the deployment can be referenced once created
* `configuration`: the configuration file, plus any imported files (templates,
schemas, helper files used by the templates, etc.).

Creating, updating or deleting a deployment creates a new manifest for the
deployment. When deleting a deployment, the deployment is first updated to
an empty manifest containing no resources, and then removed from the system.

Deployments are available at the HTTP endpoint:

```
http://manager-service/deployments
```

### Manifests

A manifest is created for a deployment every time it is changed. It contains
three key components:

* `inputConfig`: the original input configuration for the manifest
* `expandedConfig`: the expanded configuration describing only primitive resources
* `layout`: the hierarchical structure of the configuration

Manifests are available at the HTTP endpoint:

```
http://manager-service/deployments/<deployment>/manifests
```

#### Expanded configuration

Given a new `inputConfig`, DM expands all template invocations recursively,
until the result is a flat set of primitive resources. This final set is stored
as the `expandedConfig` and is used to instantiate the primitive resources.

#### Layout

Using templates, callers can build rich, deeply hierarchical architectures in
their configurations. Expansion flattens these hierarchies to simplify the process
of instantiating the primitive resources. However, the structural information
contained in the original configuration has many potential uses, so rather than
discard it, DM preserves it in the form of a `layout`.

The `layout` looks a lot like the original configuration. It is a `YAML` file
that describes a list of resources. Each resource contains the `name`, `type` 
and `properties` from the original configuration, plus a list of nested resources
discovered during expansion. The resulting structure looks like this:

* name: name of the resource
* type: type of the resource
* properties: properties of the resource, set only for templates
* resources: sub-resources from expansion, set only for templates

Here's an example of a layout:

```
resources:
- name: rs
  type: replicatedservice.py
  propertes:
    replicas: 2
  resources:
  - name: rs-rc
    type: ReplicationController
  - name: rs-service
    type: Service
```

In this example, the top level resource is a replicated service named `rs`,
defined by the template named `replicatedservice.py`. Expansion produced the
two nested resources: a replication controller named `rs-rc`, and a service
named `rs-service`.

Using the layout, callers can discover that `rs-rc` and `rs-service` are part
of the replicated service named `rs`. More importantly, if `rs` was created by
the expansion of a larger configuration, such as one that described a complete
application, callers could discover that `rs-rc` and `rs-service` were part of
the application, and perhaps even that they were part of a RabbitMQ cluster in
the application's mid-tier.

### Types
The types API provides information about types used in the cluster.

It can be used to list all known types used by active deployments:

```
http://manager-service/types
```

Or to list all active instances of a specific type in the cluster:

```
http://manager-service/types/<type>/instances
```

Passing `all` as the type name shows all instances of all types in the
cluster. The following information is reported for type instances:

* name: name of resource
* type: type of resource
* deployment: name of deployment in which the resource resides
* manifest: name of manifest in which the resource configuration resides
* path: JSON path to the entry for the resource in the manifest layout

## Architecture

The Deployment Manager service is manages deployments within a Kubernetes
cluster. It has three major components. The following diagram illustrates the
components and the relationships between them.

![Architecture Diagram](architecture.png "Architecture Diagram")

Currently, there are two caveats in the service implementation:

* Synchronous API: the API currently blocks on all processing for
  a deployment request. In the future, this design will change to an
  asynchronous operation-based mode.
* In-memory state: the service currently stores all state in memory, 
  so it will lose all knowledge of deployments and related objects on restart.
  In the future, the service will persist all state in the cluster.

### Manager

The `manager` service acts as both the API server and the workflow engine for
processing deployments. It handles a `POST` to the `/deployments` collection as
follows:

1. Create a new deployment with a manifest containing `inputConfig` from the
   user request
1. Call out to the `expandybird` service to expand the `inputConfig`
1. Store the resulting `expandedConfig` and `layout`
1. Call out to the `resourcifier` service to instantiate the primitive resources
described by the `expandedConfig`
1. Respond with success or error messages to the original API request

`GET`, `PUT` and `DELETE` operations are processed in a similar manner, except
that:

* No expansion is performed for `GET` or `DELETE`
* The primitive resources are updated for `PUT` and deleted for `DELETE`
 
The manager is responsible for saving the information associated with
deployments, manifests, type instances, and other resources in the Deployment
Manager model.

### Expandybird

The `expandybird` service takes in a configuration, performs all necesary
template expansions, and returns the resulting flat configuration and layout.
It is completely stateless.

Because templates are written in Python or Jinja, the actual expansion process
is performed in a sub-process that runs a Python interpreter. A new sub-process
is created for every request to `expandybird`.

Currently, expansion is not sandboxed, but templates should be reproducable,
hermetically sealed entities. Future designs may therefore introduce a sandbox to
limit external interaction, such as network or disk access, during expansion.

### Resourcifier

The `resourcifier` service takes in a flat expanded configuration describing
only primitive resources, and makes the necessary `kubectl` calls to process
them. It is totally stateless, and handles requests synchronously. 

The `resourcifier` runs `kubectl` in a sub-process within its container. A new
sub-process is created for every request to `resourcifier`.

It returns either success or error messages encountered during resource processing.
