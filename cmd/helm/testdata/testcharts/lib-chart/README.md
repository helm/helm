# Common: The Helm Helper Chart

This chart is designed to make it easier for you to build and maintain Helm
charts.

It provides utilities that reflect best practices of Kubernetes chart development,
making it faster for you to write charts.

## Tips

A few tips for working with Common:

- Be careful when using functions that generate random data (like `common.fullname.unique`).
  They may trigger unwanted upgrades or have other side effects.

In this document, we use `release-name` as the name of the release.

## Resource Kinds

Kubernetes defines a variety of resource kinds, from `Secret` to `StatefulSet`.
We define some of the most common kinds in a way that lets you easily work with
them.

The resource kind templates are designed to make it much faster for you to
define _basic_ versions of these resources. They allow you to extend and modify
just what you need, without having to copy around lots of boilerplate.

To make use of these templates you must define a template that will extend the
base template (though it can be empty). The name of this template is then passed
to the base template, for example:

```yaml
{{- template "common.service" (list . "mychart.service") -}}
{{- define "mychart.service" -}}
## Define overrides for your Service resource here, e.g.
# metadata:
#   labels:
#     custom: label
# spec:
#   ports:
#   - port: 8080
{{- end -}}
```

Note that the `common.service` template defines two parameters:

  - The root context (usually `.`)
  - A template name containing the service definition overrides

A limitation of the Go template library is that a template can only take a
single argument. The `list` function is used to workaround this by constructing
a list or array of arguments that is passed to the template.

The `common.service` template is responsible for rendering the templates with
the root context and merging any overrides. As you can see, this makes it very
easy to create a basic `Service` resource without having to copy around the
standard metadata and labels.

Each implemented base resource is described in greater detail below.

### `common.service`

The `common.service` template creates a basic `Service` resource with the
following defaults:

- Service type (ClusterIP, NodePort, LoadBalancer) made configurable by `.Values.service.type`
- Named port `http` configured on port 80
- Selector set to `app.kubernetes.io/name: {{ template "common.name" }}, app.kubernetes.io/instance: {{ .Release.Name | quote }}` to match the default used in the `Deployment` resource

Example template:

```yaml
{{- template "common.service" (list . "mychart.mail.service") -}}
{{- define "mychart.mail.service" -}}
metadata:
  name: {{ template "common.fullname" . }}-mail # overrides the default name to add a suffix
  labels:                                       # appended to the labels section
    protocol: mail
spec:
  ports:                                        # composes the `ports` section of the service definition.
  - name: smtp
    port: 25
    targetPort: 25
  - name: imaps
    port: 993
    targetPort: 993
  selector:                                     # this is appended to the default selector
    protocol: mail
{{- end -}}
---
{{ template "common.service" (list . "mychart.web.service") -}}
{{- define "mychart.web.service" -}}
metadata:
  name: {{ template "common.fullname" . }}-www  # overrides the default name to add a suffix
  labels:                                       # appended to the labels section
    protocol: www
spec:
  ports:                                        # composes the `ports` section of the service definition.
  - name: www
    port: 80
    targetPort: 8080
{{- end -}}
```

The above template defines _two_ services: a web service and a mail service.

The most important part of a service definition is the `ports` object, which
defines the ports that this service will listen on. Most of the time,
`selector` is computed for you. But you can replace it or add to it.

The output of the example above is:

```yaml
apiVersion: v1
kind: Service
metadata:
  labels:
    app.kubernetes.io/name: service
    helm.sh/chart: service-0.1.0
    app.kubernetes.io/managed-by: Helm
    protocol: mail
    app.kubernetes.io/instance: release-name
  name: release-name-service-mail
spec:
  ports:
  - name: smtp
    port: 25
    targetPort: 25
  - name: imaps
    port: 993
    targetPort: 993
  selector:
    app.kubernetes.io/name: service
    app.kubernetes.io/instance: release-name
    protocol: mail
  type: ClusterIP
---
apiVersion: v1
kind: Service
metadata:
  labels:
    app.kubernetes.io/name: service
    helm.sh/chart: service-0.1.0
    app.kubernetes.io/managed-by: Helm
    protocol: www
    app.kubernetes.io/instance: release-name
  name: release-name-service-www
spec:
  ports:
  - name: www
    port: 80
    targetPort: 8080
  type: ClusterIP
```

## `common.deployment`

The `common.deployment` template defines a basic `Deployment`. Underneath the
hood, it uses `common.container` (see next section).

By default, the pod template within the deployment defines the labels `app: {{ template "common.name" . }}`
and `release: {{ .Release.Name | quote }` as this is also used as the selector. The
standard set of labels are not used as some of these can change during upgrades,
which causes the replica sets and pods to not correctly match.

Example use:

```yaml
{{- template "common.deployment" (list . "mychart.deployment") -}}
{{- define "mychart.deployment" -}}
## Define overrides for your Deployment resource here, e.g.
spec:
  replicas: {{ .Values.replicaCount }}
{{- end -}}
```

## `common.container`

The `common.container` template creates a basic `Container` spec to be used
within a `Deployment` or `ReplicaSet`. It holds the following defaults:

- The name is set to the chart name
- Uses `.Values.image` to describe the image to run, with the following spec:
  ```yaml
  image:
    repository: nginx
    tag: stable
    pullPolicy: IfNotPresent
  ```
- Exposes the named port `http` as port 80
- Lays out the compute resources using `.Values.resources`

Example use:

```yaml
{{- template "common.deployment" (list . "mychart.deployment") -}}
{{- define "mychart.deployment" -}}
## Define overrides for your Deployment resource here, e.g.
spec:
  template:
    spec:
      containers:
      - {{ template "common.container" (list . "mychart.deployment.container") }}
{{- end -}}
{{- define "mychart.deployment.container" -}}
## Define overrides for your Container here, e.g.
livenessProbe:
  httpGet:
    path: /
    port: 80
readinessProbe:
  httpGet:
    path: /
    port: 80
{{- end -}}
```

The above example creates a `Deployment` resource which makes use of the
`common.container` template to populate the PodSpec's container list. The usage
of this template is similar to the other resources, you must define and
reference a template that contains overrides for the container object.

The most important part of a container definition is the image you want to run.
As mentioned above, this is derived from `.Values.image` by default. It is a
best practice to define the image, tag and pull policy in your charts' values as
this makes it easy for an operator to change the image registry, or use a
specific tag or version. Another example of configuration that should be exposed
to chart operators is the container's required compute resources, as this is
also very specific to an operators environment. An example `values.yaml` for
your chart could look like:

```yaml
image:
  repository: nginx
  tag: stable
  pullPolicy: IfNotPresent
resources:
  limits:
    cpu: 100m
    memory: 128Mi
  requests:
    cpu: 100m
    memory: 128Mi
```

The output of running the above values through the earlier template is:

```yaml
apiVersion: extensions/v1beta1
kind: Deployment
metadata:
  labels:
    app.kubernetes.io/name: deployment
    helm.sh/chart: deployment-0.1.0
    app.kubernetes.io/managed-by: Helm
    app.kubernetes.io/instance: release-name
  name: release-name-deployment
spec:
  template:
    metadata:
      labels:
        app.kubernetes.io/name: deployment
    spec:
      containers:
      - image: nginx:stable
        imagePullPolicy: IfNotPresent
        livenessProbe:
          httpGet:
            path: /
            port: 80
        name: deployment
        ports:
        - containerPort: 80
          name: http
        readinessProbe:
          httpGet:
            path: /
            port: 80
        resources:
          limits:
            cpu: 100m
            memory: 128Mi
          requests:
            cpu: 100m
            memory: 128Mi
```

## `common.configmap`

The `common.configmap` template creates an empty `ConfigMap` resource that you
can override with your configuration.

Example use:

```yaml
{{- template "common.configmap" (list . "mychart.configmap") -}}
{{- define "mychart.configmap" -}}
data:
  zeus: cat
  athena: cat
  julius: cat
  one: |-
    {{ .Files.Get "file1.txt" }}
{{- end -}}
```

Output:

```yaml
apiVersion: v1
data:
  athena: cat
  julius: cat
  one: This is a file.
  zeus: cat
kind: ConfigMap
metadata:
  labels:
    app.kubernetes.io/name: configmap
    helm.sh/chart: configmap-0.1.0
    app.kubernetes.io/managed-by: Helm
    app.kubernetes.io/instance: release-name
  name: release-name-configmap
```

## `common.secret`

The `common.secret` template creates an empty `Secret` resource that you
can override with your secrets.

Example use:

```yaml
{{- template "common.secret" (list . "mychart.secret") -}}
{{- define "mychart.secret" -}}
data:
  zeus: {{ print "cat" | b64enc }}
  athena: {{ print "cat" | b64enc }}
  julius: {{ print "cat" | b64enc }}
  one: |-
    {{ .Files.Get "file1.txt" | b64enc }}
{{- end -}}
```

Output:

```yaml
apiVersion: v1
data:
  athena: Y2F0
  julius: Y2F0
  one: VGhpcyBpcyBhIGZpbGUuCg==
  zeus: Y2F0
kind: Secret
metadata:
  labels:
    app.kubernetes.io/name: secret
    helm.sh/chart: secret-0.1.0
    app.kubernetes.io/managed-by: Helm
    app.kubernetes.io/instance: release-name
  name: release-name-secret
type: Opaque
```

## `common.ingress`

The `common.ingress` template is designed to give you a well-defined `Ingress`
resource, that can be configured using `.Values.ingress`. An example values file
that can be used to configure the `Ingress` resource is:

```yaml
ingress:
  hosts:
  - chart-example.local
  annotations:
    kubernetes.io/ingress.class: nginx
    kubernetes.io/tls-acme: "true"
  tls:
  - secretName: chart-example-tls
    hosts:
    - chart-example.local
```

Example use:

```yaml
{{- template "common.ingress" (list . "mychart.ingress") -}}
{{- define "mychart.ingress" -}}
{{- end -}}
```

Output:

```yaml
apiVersion: extensions/v1beta1
kind: Ingress
metadata:
  annotations:
    kubernetes.io/ingress.class: nginx
    kubernetes.io/tls-acme: "true"
  labels:
    app.kubernetes.io/name: ingress
    helm.sh/chart: ingress-0.1.0
    app.kubernetes.io/managed-by: Helm
    app.kubernetes.io/instance: release-name
  name: release-name-ingress
spec:
  rules:
  - host: chart-example.local
    http:
      paths:
      - backend:
          serviceName: release-name-ingress
          servicePort: 80
        path: /
  tls:
  - hosts:
    - chart-example.local
    secretName: chart-example-tls
```

## `common.persistentvolumeclaim`

`common.persistentvolumeclaim` can be used to easily add a
`PersistentVolumeClaim` resource to your chart that can be configured using
`.Values.persistence`:

|           Value           |                                               Description                                               |
| ------------------------- | ------------------------------------------------------------------------------------------------------- |
| persistence.enabled       | Whether or not to claim a persistent volume. If false, `common.volume.pvc` will use an emptyDir instead |
| persistence.storageClass  | `StorageClass` name                                                                                     |
| persistence.accessMode    | Access mode for persistent volume                                                                       |
| persistence.size          | Size of persistent volume                                                                               |
| persistence.existingClaim | If defined, `PersistentVolumeClaim` is not created and `common.volume.pvc` helper uses this claim       |

An example values file that can be used to configure the
`PersistentVolumeClaim` resource is:

```yaml
persistence:
  enabled: true
  storageClass: fast
  accessMode: ReadWriteOnce
  size: 8Gi
```

Example use:

```yaml
{{- template "common.persistentvolumeclaim" (list . "mychart.persistentvolumeclaim") -}}
{{- define "mychart.persistentvolumeclaim" -}}
{{- end -}}
```

Output:

```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  labels:
    app.kubernetes.io/name: persistentvolumeclaim
    helm.sh/chart: persistentvolumeclaim-0.1.0
    app.kubernetes.io/managed-by: Helm
    app.kubernetes.io/instance: release-name
  name: release-name-persistentvolumeclaim
spec:
  accessModes:
  - ReadWriteOnce
  resources:
    requests:
      storage: 8Gi
  storageClassName: "fast"
```

## Partial API Objects

When writing Kubernetes resources, you may find the following helpers useful to
construct parts of the spec.

### EnvVar

Use the EnvVar helpers within a container spec to simplify specifying key-value
environment variables or referencing secrets as values.

Example Use:

```yaml
{{- template "common.deployment" (list . "mychart.deployment") -}}
{{- define "mychart.deployment" -}}
spec:
  template:
    spec:
      containers:
      - {{ template "common.container" (list . "mychart.deployment.container") }}
{{- end -}}
{{- define "mychart.deployment.container" -}}
{{- $fullname := include "common.fullname" . -}}
env:
- {{ template "common.envvar.value" (list "ZEUS" "cat") }}
- {{ template "common.envvar.secret" (list "ATHENA" "secret-name" "athena") }}
{{- end -}}
```

Output:

```yaml
...
    spec:
      containers:
      - env:
        - name: ZEUS
          value: cat
        - name: ATHENA
          valueFrom:
            secretKeyRef:
              key: athena
              name: secret-name
...
```

### Volume

Use the Volume helpers within a `Deployment` spec to help define ConfigMap and
PersistentVolumeClaim volumes.

Example Use:

```yaml
{{- template "common.deployment" (list . "mychart.deployment") -}}
{{- define "mychart.deployment" -}}
spec:
  template:
    spec:
      volumes:
      - {{ template "common.volume.configMap" (list "config" "configmap-name") }}
      - {{ template "common.volume.pvc" (list "data" "pvc-name" .Values.persistence) }}
{{- end -}}
```

Output:

```yaml
...
    spec:
      volumes:
      - configMap:
          name: configmap-name
        name: config
      - name: data
        persistentVolumeClaim:
          claimName: pvc-name
...
```

The `common.volume.pvc` helper uses the following configuration from the `.Values.persistence` object:

|           Value           |                      Description                      |
| ------------------------- | ----------------------------------------------------- |
| persistence.enabled       | If false, creates an `emptyDir` instead               |
| persistence.existingClaim | If set, uses this instead of the passed in claim name |

## Utilities

### `common.fullname`

The `common.fullname` template generates a name suitable for the `name:` field
in Kubernetes metadata. It is used like this:

```yaml
name: {{ template "common.fullname" . }}
```

The following different values can influence it:

```yaml
# By default, fullname uses '{{ .Release.Name }}-{{ .Chart.Name }}'. This
# overrides that and uses the given string instead.
fullnameOverride: "some-name"

# This adds a prefix
fullnamePrefix: "pre-"
# This appends a suffix
fullnameSuffix: "-suf"

# Global versions of the above
global:
  fullnamePrefix: "pp-"
  fullnameSuffix: "-ps"
```

Example output:

```yaml
---
# with the values above
name: pp-pre-some-name-suf-ps

---
# the default, for release "happy-panda" and chart "wordpress"
name: happy-panda-wordpress
```

Output of this function is truncated at 54 characters, which leaves 9 additional
characters for customized overriding. Thus you can easily extend this name
in your own charts:

```yaml
{{- define "my.fullname" -}}
  {{ template "common.fullname" . }}-my-stuff
{{- end -}}
```

### `common.fullname.unique`

The `common.fullname.unique` variant of fullname appends a unique seven-character
sequence to the end of the common name field.

This takes all of the same parameters as `common.fullname`

Example template:

```yaml
uniqueName: {{ template "common.fullname.unique" . }}
```

Example output:

```yaml
uniqueName: release-name-fullname-jl0dbwx
```

It is also impacted by the prefix and suffix definitions, as well as by
`.Values.fullnameOverride`

Note that the effective maximum length of this function is 63 characters, not 54.

### `common.name`

The `common.name` template generates a name suitable for the `app` label. It is used like this:

```yaml
app: {{ template "common.name" . }}
```

The following different values can influence it:

```yaml
# By default, name uses '{{ .Chart.Name }}'. This
# overrides that and uses the given string instead.
nameOverride: "some-name"

# This adds a prefix
namePrefix: "pre-"
# This appends a suffix
nameSuffix: "-suf"

# Global versions of the above
global:
  namePrefix: "pp-"
  nameSuffix: "-ps"
```

Example output:

```yaml
---
# with the values above
name: pp-pre-some-name-suf-ps

---
# the default, for chart "wordpress"
name: wordpress
```

Output of this function is truncated at 54 characters, which leaves 9 additional
characters for customized overriding. Thus you can easily extend this name
in your own charts:

```yaml
{{- define "my.name" -}}
  {{ template "common.name" . }}-my-stuff
{{- end -}}
```

### `common.metadata`

The `common.metadata` helper generates the `metadata:` section of a Kubernetes
resource.

This takes three objects:
  - .top: top context
  - .fullnameOverride: override the fullname with this name
  - .metadata
    - .labels: key/value list of labels
    - .annotations: key/value list of annotations
    - .hook: name(s) of hook(s)

It generates standard labels, annotations, hooks, and a name field.

Example template:

```yaml
{{ template "common.metadata" (dict "top" . "metadata" .Values.bio) }}
---
{{ template "common.metadata" (dict "top" . "metadata" .Values.pet "fullnameOverride" .Values.pet.fullnameOverride) }}
```

Example values:

```yaml
bio:
  name: example
  labels:
    first: matt
    last: butcher
    nick: technosophos
  annotations:
    format: bio
    destination: archive
  hook: pre-install

pet:
  fullnameOverride: Zeus

```

Example output:

```yaml
metadata:
  name: release-name-metadata
  labels:
    app.kubernetes.io/name: metadata
    app.kubernetes.io/managed-by: "Helm"
    app.kubernetes.io/instance: "release-name"
    helm.sh/chart: metadata-0.1.0
    first: "matt"
    last: "butcher"
    nick: "technosophos"
  annotations:
    "destination": "archive"
    "format": "bio"
    "helm.sh/hook": "pre-install"
---
metadata:
  name: Zeus
  labels:
    app.kubernetes.io/name: metadata
    app.kubernetes.io/managed-by: "Helm"
    app.kubernetes.io/instance: "release-name"
    helm.sh/chart: metadata-0.1.0
  annotations:
```

Most of the common templates that define a resource type (e.g. `common.configmap`
or `common.job`) use this to generate the metadata, which means they inherit
the same `labels`, `annotations`, `nameOverride`, and `hook` fields.

### `common.labelize`

`common.labelize` turns a map into a set of labels.

Example template:

```yaml
{{- $map := dict "first" "1" "second" "2" "third" "3" -}}
{{- template "common.labelize" $map -}}
```

Example output:

```yaml
first: "1"
second: "2"
third: "3"
```

### `common.labels.standard`

`common.labels.standard` prints the standard set of labels.

Example usage:

```
{{ template "common.labels.standard" . }}
```

Example output:

```yaml
app.kubernetes.io/name: labelizer
app.kubernetes.io/managed-by: "Tiller"
app.kubernetes.io/instance: "release-name"
helm.sh/chart: labelizer-0.1.0
```

### `common.hook`

The `common.hook` template is a convenience for defining hooks.

Example template:

```yaml
{{ template "common.hook" "pre-install,post-install" }}
```

Example output:

```yaml
"helm.sh/hook": "pre-install,post-install"
```

### `common.chartref`

The `common.chartref` helper prints the chart name and version, escaped to be
legal in a Kubernetes label field.

Example template:

```yaml
chartref: {{ template "common.chartref" . }}
```

For the chart `foo` with version `1.2.3-beta.55+1234`, this will render:

```yaml
chartref: foo-1.2.3-beta.55_1234
```

(Note that `+` is an illegal character in label values)
