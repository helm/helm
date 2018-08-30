# Hooks

Helm provides a _hook_ mechanism to allow chart developers to intervene
at certain points in a release's life cycle. For example, you can use
hooks to:

- Load a ConfigMap or Secret during install before any other charts are
  loaded.
- Execute a Job to back up a database before installing a new chart,
  and then execute a second job after the upgrade in order to restore
  data.
- Run a Job before deleting a release to gracefully take a service out
  of rotation before removing it.

Hooks work like regular templates, but they have special annotations
that cause Helm to utilize them differently. In this section, we cover
the basic usage pattern for hooks.

Hooks are declared as an annotation in the metadata section of a manifest:

```yaml
apiVersion: ...
kind: ....
metadata:
  annotations:
    "helm.sh/hook": "pre-install"
# ...
```

## The Available Hooks

The following hooks are defined:

- pre-install: Executes after templates are rendered, but before any
  resources are created in Kubernetes.
- post-install: Executes after all resources are loaded into Kubernetes
- pre-delete: Executes on a deletion request before any resources are
  deleted from Kubernetes.
- post-delete: Executes on a deletion request after all of the release's
  resources have been deleted.
- pre-upgrade: Executes on an upgrade request after templates are
  rendered, but before any resources are loaded into Kubernetes (e.g.
  before a Kubernetes apply operation).
- post-upgrade: Executes on an upgrade after all resources have been
  upgraded.
- pre-rollback: Executes on a rollback request after templates are
  rendered, but before any resources have been rolled back.
- post-rollback: Executes on a rollback request after all resources
  have been modified.
- crd-install: Adds CRD resources before any other checks are run. This is used
  only on CRD definitions that are used by other manifests in the chart.

## Hooks and the Release Lifecycle

Hooks allow you, the chart developer, an opportunity to perform
operations at strategic points in a release lifecycle. For example,
consider the lifecycle for a `helm install`. By default, the lifecycle
looks like this:

1. User runs `helm install foo`
2. Chart is loaded into Tiller
3. After some verification, Tiller renders the `foo` templates
4. Tiller loads the resulting resources into Kubernetes
5. Tiller returns the release name (and other data) to the client
6. The client exits

Helm defines two hooks for the `install` lifecycle: `pre-install` and
`post-install`. If the developer of the `foo` chart implements both
hooks, the lifecycle is altered like this:

1. User runs `helm install foo`
2. Chart is loaded into Tiller
3. After some verification, Tiller renders the `foo` templates
4. Tiller prepares to execute the `pre-install` hooks (loading hook resources into
   Kubernetes)
5. Tiller sorts hooks by weight (assigning a weight of 0 by default) and by name for those hooks with the same weight in ascending order.
6. Tiller then loads the hook with the lowest weight first (negative to positive)
7. Tiller waits until the hook is "Ready" (except for CRDs)
8. Tiller loads the resulting resources into Kubernetes. Note that if the `--wait` 
flag is set, Tiller will wait until all resources are in a ready state
and will not run the `post-install` hook until they are ready.
9. Tiller executes the `post-install` hook (loading hook resources)
10. Tiller waits until the hook is "Ready"
11. Tiller returns the release name (and other data) to the client
12. The client exits

What does it mean to wait until a hook is ready? This depends on the
resource declared in the hook. If the resources is a `Job` kind, Tiller
will wait until the job successfully runs to completion. And if the job
fails, the release will fail. This is a _blocking operation_, so the
Helm client will pause while the Job is run.

For all other kinds, as soon as Kubernetes marks the resource as loaded
(added or updated), the resource is considered "Ready". When many
resources are declared in a hook, the resources are executed serially. If they
have hook weights (see below), they are executed in weighted order. Otherwise,
ordering is not guaranteed. (In Helm 2.3.0 and after, they are sorted
alphabetically. That behavior, though, is not considered binding and could change
in the future.) It is considered good practice to add a hook weight, and set it
to `0` if weight is not important.


### Hook resources are not managed with corresponding releases

The resources that a hook creates are not tracked or managed as part of the
release. Once Tiller verifies that the hook has reached its ready state, it
will leave the hook resource alone.

Practically speaking, this means that if you create resources in a hook, you
cannot rely upon `helm delete` to remove the resources. To destroy such
resources, you need to either write code to perform this operation in a `pre-delete`
or `post-delete` hook or add `"helm.sh/hook-delete-policy"` annotation to the hook template file.

## Writing a Hook

Hooks are just Kubernetes manifest files with special annotations in the
`metadata` section. Because they are template files, you can use all of
the normal template features, including reading `.Values`, `.Release`,
and `.Template`.

For example, this template, stored in `templates/post-install-job.yaml`,
declares a job to be run on `post-install`:

```yaml
apiVersion: batch/v1
kind: Job
metadata:
  name: "{{.Release.Name}}"
  labels:
    app.kubernetes.io/managed-by: {{.Release.Service | quote }}
    app.kubernetes.io/instance: {{.Release.Name | quote }}
    helm.sh/chart: "{{.Chart.Name}}-{{.Chart.Version}}"
  annotations:
    # This is what defines this resource as a hook. Without this line, the
    # job is considered part of the release.
    "helm.sh/hook": post-install
    "helm.sh/hook-weight": "-5"
    "helm.sh/hook-delete-policy": hook-succeeded
spec:
  template:
    metadata:
      name: "{{.Release.Name}}"
      labels:
        app.kubernetes.io/managed-by: {{.Release.Service | quote }}
        app.kubernetes.io/instance: {{.Release.Name | quote }}
        helm.sh/chart: "{{.Chart.Name}}-{{.Chart.Version}}"
    spec:
      restartPolicy: Never
      containers:
      - name: post-install-job
        image: "alpine:3.3"
        command: ["/bin/sleep","{{default "10" .Values.sleepyTime}}"]

```

What makes this template a hook is the annotation:

```
  annotations:
    "helm.sh/hook": post-install
```

One resource can implement multiple hooks:

```
  annotations:
    "helm.sh/hook": post-install,post-upgrade
```

Similarly, there is no limit to the number of different resources that
may implement a given hook. For example, one could declare both a secret
and a config map as a pre-install hook.

When subcharts declare hooks, those are also evaluated. There is no way
for a top-level chart to disable the hooks declared by subcharts.

It is possible to define a weight for a hook which will help build a
deterministic executing order. Weights are defined using the following annotation:

```
  annotations:
    "helm.sh/hook-weight": "5"
```

Hook weights can be positive or negative numbers but must be represented as
strings. When Tiller starts the execution cycle of hooks of a particular kind (ex. the `pre-install` hooks or `post-install` hooks, etc.) it will sort those hooks in ascending order.

It is also possible to define policies that determine when to delete corresponding hook resources. Hook deletion policies are defined using the following annotation:

```
  annotations:
    "helm.sh/hook-delete-policy": hook-succeeded
```

You can choose one or more defined annotation values:

* `"hook-succeeded"` specifies Tiller should delete the hook after the hook is successfully executed.
* `"hook-failed"` specifies Tiller should delete the hook if the hook failed during execution.
* `"before-hook-creation"` specifies Tiller should delete the previous hook before the new hook is launched.

### Defining a CRD with the `crd-install` Hook

Custom Resource Definitions (CRDs) are a special kind in Kubernetes. They provide
a way to define other kinds.

On occasion, a chart needs to both define a kind and then use it. This is done
with the `crd-install` hook.

The `crd-install` hook is executed very early during an installation, before
the rest of the manifests are verified. CRDs can be annotated with this hook so
that they are installed before any instances of that CRD are referenced. In this
way, when verification happens later, the CRDs will be available.

Here is an example of defining a CRD with a hook, and an instance of the CRD:

```yaml
apiVersion: apiextensions.k8s.io/v1beta1
kind: CustomResourceDefinition
metadata:
  name: crontabs.stable.example.com
  annotations:
    "helm.sh/hook": crd-install
spec:
  group: stable.example.com
  version: v1
  scope: Namespaced
  names:
    plural: crontabs
    singular: crontab
    kind: CronTab
    shortNames:
    - ct
```

And:

```yaml
apiVersion: stable.example.com/v1
kind: CronTab
metadata:
  name: {{ .Release.Name }}-inst
```

Both of these can now be in the same chart, provided that the CRD is correctly
annotated.

### Automatically delete hook from previous release

When helm release being updated it is possible, that hook resource already exists in cluster. By default helm will try to create resource and fail with `"... already exists"` error.

One might choose `"helm.sh/hook-delete-policy": "before-hook-creation"` over `"helm.sh/hook-delete-policy": "hook-succeeded,hook-failed"` because:

* It is convenient to keep failed hook job resource in kubernetes for example for manual debug.
* It may be necessary to keep succeeded hook resource in kubernetes for some reason.
* At the same time it is not desirable to do manual resource deletion before helm release upgrade.

`"helm.sh/hook-delete-policy": "before-hook-creation"` annotation on hook causes tiller to remove the hook from previous release if there is one before the new hook is launched and can be used with another policy.
