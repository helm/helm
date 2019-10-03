# Custom Resource Definitions

This section of the Best Practices Guide deals with creating and using Custom Resource Definition
objects.

When working with Custom Resource Definitions (CRDs), it is important to distinguish
two different pieces:

- There is a declaration of a CRD. This is the YAML file that has the kind `CustomResourceDefinition`
- Then there are resources that _use_ the CRD. Say a CRD defines `foo.example.com/v1`. Any resource
  that has `apiVersion: example.com/v1` and kind `Foo` is a resource that uses the CRD.

## Install a CRD Declaration Before Using the Resource

Helm is optimized to load as many resources into Kubernetes as fast as possible.
By design, Kubernetes can take an entire set of manifests and bring them all
online (this is called the reconciliation loop).

But there's a difference with CRDs.

For a CRD, the declaration must be registered before any resources of that CRDs
kind(s) can be used. And the registration process sometimes takes a few seconds.

### Method 1: Separate Charts

One way to do this is to put the CRD definition in one chart, and then put any
resources that use that CRD in _another_ chart.

In this method, each chart must be installed separately.

### Method 2: Crd-install Hooks

To package the two together, add a `crd-install` hook to the CRD definition so
that it is fully installed before the rest of the chart is executed.

Note that if you create the CRD with a `crd-install` hook, that CRD definition
will not be deleted when `helm delete` is run.
