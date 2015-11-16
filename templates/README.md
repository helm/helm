# Kubernetes Template Registry

Welcome to the Kubernetes Template Registry!

This registry holds Deployment Manager
[templates](https://github.com/kubernetes/deployment-manager/tree/master/docs/design/design.md#templates)
that you can use to deploy Kubernetes resources.

For more information about installing and using Deployment Manager, see its
[README.md](https://github.com/kubernetes/deployment-manager/tree/master/README.md).

## Organization

The Kubernetes Template Registry is organized as a flat list of template
directories. Templates are versioned. The versions of a template live in version
directories under its template directory. 

For more information about Deployment Manager template registries, including
directory structure and template versions, see the
[template registry documentation](https://github.com/kubernetes/deployment-manager/tree/master/docs/templates/registry.md)

## Contributing

The Kubernetes Template Registry follows the
[git setup](https://github.com/kubernetes/kubernetes/blob/master/docs/devel/development.md#git-setup)
used by Kubernetes.

You will also need to have a Contributor License Agreement on file, as described
in [CONTRIBUTING.md](../CONTRIBUTING.md).
