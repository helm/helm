# Labels and Annotations

This part of the Best Practices Guide discusses the best practices for using
labels and annotations in your chart.

## Is it a Label or an Annotation?

An item of metadata should be a label under the following conditions:

- It is used by Kubernetes to identify this resource
- It is useful to expose to operators for the purpose of querying the system.

For example, we suggest using `helm.sh/chart: NAME-VERSION` as a label so that operators
can conveniently find all of the instances of a particular chart to use.

If an item of metadata is not used for querying, it should be set as an annotation
instead.

Helm hooks are always annotations.

## Standard Labels

The following table defines common labels that Helm charts use. Helm itself never requires that a particular label be present. Labels that are marked REC
are recommended, and _should_ be placed onto a chart for global consistency. Those marked OPT are optional. These are idiomatic or commonly in use, but are not relied upon frequently for operational purposes.

Name|Status|Description
-----|------|----------
`app.kubernetes.io/name` | REC | This should be the app name, reflecting the entire app. Usually `{{ template "name" . }}` is used for this. This is used by many Kubernetes manifests, and is not Helm-specific.
`helm.sh/chart` | REC | This should be the chart name and version: `{{ .Chart.Name }}-{{ .Chart.Version \| replace "+" "_" }}`.
`app.kubernetes.io/managed-by` | REC | This should always be set to `{{ .Release.Service }}`. It is for finding all things managed by Tiller.
`app.kubernetes.io/instance` | REC | This should be the `{{ .Release.Name }}`. It aids in differentiating between different instances of the same application.
`app.kubernetes.io/version` | OPT | The version of the app and can be set to `{{ .Chart.AppVersion }}`.
`app.kubernetes.io/component` | OPT | This is a common label for marking the different roles that pieces may play in an application. For example, `app.kubernetes.io/component: frontend`.
`app.kubernetes.io/part-of` | OPT | When multiple charts or pieces of software are used together to make one application. For example, application software and a database to produce a website. This can be set to the top level application being supported.

You can find more information on the Kubernetes labels, prefixed with `app.kubernetes.io`, in the [Kubernetes documentation](https://kubernetes.io/docs/concepts/overview/working-with-objects/common-labels/).
