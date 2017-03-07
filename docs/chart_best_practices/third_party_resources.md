# Third Party Resources

This section of the Best Practices Guide deals with creating and using Third Party Resource
objects.

When working with Third Party Resources (TPRs), it is important to distinguish
two different pieces:

- There is a declaration of a TPR. This is the YAML file that has the kind `ThirdPartyResource`
- Then there are resources that _use_ the TPR. Say a TPR defines `foo.example.com/v1`. Any resource
  that has `apiVersion: example.com/v1` and kind `Foo` is a resource that uses the
  TPR.

## Install a TPR Declaration Before Using the Resource

Helm is optimized to load as many resources into Kubernetes as fast as possible.
By design, Kubernetes can take an entire set of manifests and bring them all
online (this is called the reconciliation loop).

But there's a difference with TPRs.

For a TPR, the declaration must be registered before any resources of that TPRs
kind(s) can be used. And the registration process sometimes takes a few seconds.

### Method 1: Separate Charts

One way to do this is to put the TPR definition in one chart, and then put any
resources that use that TPR in _another_ chart.

In this method, each chart must be installed separately.

### Method 2: Pre-install Hooks

To package the two together, add a `pre-install` hook to the TPR definition so
that it is fully installed before the rest of the chart is executed.

Note that if you create the TPR with a `pre-install` hook, that TPR definition
will not be deleted when `helm delete` is run.
