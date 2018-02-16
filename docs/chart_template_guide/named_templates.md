# Named Templates

It is time to move beyond one template, and begin to create others. In this section, we will see how to define _named templates_ in one file, and then use them elsewhere. A _named template_ (sometimes called a _partial_ or a _subtemplate_) is simply a template defined inside of a file, and given a name. We'll see two ways to create them, and a few different ways to use them.

In the "Flow Control" section we introduced three actions for declaring and managing templates: `define`, `template`, and `block`. In this section, we'll cover those three actions, and also introduce a special-purpose `include` function that works similarly to the `template` action.

An important detail to keep in mind when naming templates: **template names are global**. If you declare two templates with the same name, whichever one is loaded last will be the one used. Because templates in subcharts are compiled together with top-level templates, you should be careful to name your templates with _chart-specific names_.

One popular naming convention is to prefix each defined template with the name of the chart: `{{ define "mychart.labels" }}`. By using the specific chart name as a prefix we can avoid any conflicts that may arise due to two different charts that implement templates of the same name.

## Partials and `_` files

So far, we've used one file, and that one file has contained a single template. But Helm's template language allows you to create named embedded templates, that can be accessed by name elsewhere.

Before we get to the nuts-and-bolts of writing those templates, there is file naming convention that deserves mention:

* Most files in `templates/` are treated as if they contain Kubernetes manifests
* The `NOTES.txt` is one exception
* But files whose name begins with an underscore (`_`) are assumed to _not_ have a manifest inside. These files are not rendered to Kubernetes object definitions, but are available everywhere within other chart templates for use.

These files are used to store partials and helpers. In fact, when we first created `mychart`, we saw a file called `_helpers.tpl`. That file is the default location for template partials.

## Declaring and using templates with `define` and `template`

The `define` action allows us to create a named template inside of a template file. Its syntax goes like this:

```yaml
{{ define "MY.NAME" }}
  # body of template here
{{ end }}
```

For example, we can define a template to encapsulate a Kubernetes block of labels:

```yaml
{{- define "mychart.labels" }}
  labels:
    generator: helm
    date: {{ now | htmlDate }}
{{- end }}
```

Now we can embed this template inside of our existing ConfigMap, and then include it with the `template` action:

```yaml
{{- define "mychart.labels" }}
  labels:
    generator: helm
    date: {{ now | htmlDate }}
{{- end }}
apiVersion: v1
kind: ConfigMap
metadata:
  name: {{ .Release.Name }}-configmap
  {{- template "mychart.labels" }}
data:
  myvalue: "Hello World"
  {{- range $key, $val := .Values.favorite }}
  {{ $key }}: {{ $val | quote }}
  {{- end }}
```

When the template engine reads this file, it will store away the reference to `mychart.labels` until `template "mychart.labels"` is called. Then it will render that template inline. So the result will look like this:

```yaml
# Source: mychart/templates/configmap.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: running-panda-configmap
  labels:
    generator: helm
    date: 2016-11-02
data:
  myvalue: "Hello World"
  drink: "coffee"
  food: "pizza"
```

Conventionally, Helm charts put these templates inside of a partials file, usually `_helpers.tpl`. Let's move this function there:

```yaml
{{/* Generate basic labels */}}
{{- define "mychart.labels" }}
  labels:
    generator: helm
    date: {{ now | htmlDate }}
{{- end }}
```

By convention, `define` functions should have a simple documentation block (`{{/* ... */}}`) describing what they do.

Even though this definition is in `_helpers.tpl`, it can still be accessed in `configmap.yaml`:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: {{ .Release.Name }}-configmap
  {{- template "mychart.labels" }}
data:
  myvalue: "Hello World"
  {{- range $key, $val := .Values.favorite }}
  {{ $key }}: {{ $val | quote }}
  {{- end }}
```

As mentioned above, **template names are global**. As a result of this, if two templates are declared with the same name the last occurance will be the one that is used. Since templates in subcharts are compiled together with top-level templates, it is best to name your templates with _chart specific names_. A popular naming convention is to prefix each defined template with the name of the chart: `{{ define "mychart.labels" }}`.

## Setting the scope of a template

In the template we defined above, we did not use any objects. We just used functions. Let's modify our defined template to include the chart name and chart version:

```yaml
{{/* Generate basic labels */}}
{{- define "mychart.labels" }}
  labels:
    generator: helm
    date: {{ now | htmlDate }}
    chart: {{ .Chart.Name }}
    version: {{ .Chart.Version }}
{{- end }}
```

If we render this, the result will not be what we expect:

```yaml
# Source: mychart/templates/configmap.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: moldy-jaguar-configmap
  labels:
    generator: helm
    date: 2016-11-02
    chart:
    version:
```

What happened to the name and version? They weren't in the scope for our defined template. When a named template (created with `define`) is rendered, it will receive the scope passed in by the `template` call. In our example, we included the template like this:

```yaml
{{- template "mychart.labels" }}
```

No scope was passed in, so within the template we cannot access anything in `.`. This is easy enough to fix, though. We simply pass a scope to the template:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: {{ .Release.Name }}-configmap
  {{- template "mychart.labels" . }}
```

Note that we pass `.` at the end of the `template` call. We could just as easily pass `.Values` or `.Values.favorite` or whatever scope we want. But what we want is the top-level scope.

Now when we execute this template with `helm install --dry-run --debug ./mychart`, we get this:

```yaml
# Source: mychart/templates/configmap.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: plinking-anaco-configmap
  labels:
    generator: helm
    date: 2016-11-02
    chart: mychart
    version: 0.1.0
```

Now `{{ .Chart.Name }}` resolves to `mychart`, and `{{ .Chart.Version }}` resolves to `0.1.0`.

## The `include` function

Say we've defined a simple template that looks like this:

```yaml
{{- define "mychart.app" -}}
app_name: {{ .Chart.Name }}
app_version: "{{ .Chart.Version }}+{{ .Release.Time.Seconds }}"
{{- end -}}
```

Now say I want to insert this both into the `labels:` section of my template, and also the `data:` section:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: {{ .Release.Name }}-configmap
  labels:
    {{ template "mychart.app" .}}
data:
  myvalue: "Hello World"
  {{- range $key, $val := .Values.favorite }}
  {{ $key }}: {{ $val | quote }}
  {{- end }}
{{ template "mychart.app" . }}
```

The output will not be what we expect:

```yaml
# Source: mychart/templates/configmap.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: measly-whippet-configmap
  labels:
    app_name: mychart
app_version: "0.1.0+1478129847"
data:
  myvalue: "Hello World"
  drink: "coffee"
  food: "pizza"
  app_name: mychart
app_version: "0.1.0+1478129847"
```

Note that the indentation on `app_version` is wrong in both places. Why? Because the template that is substituted in has the text aligned to the right. Because `template` is an action, and not a function, there is no way to pass the output of a `template` call to other functions; the data is simply inserted inline.

To work around this case, Helm provides an alternative to `template` that will import the contents of a template into the present pipeline where it can be passed along to other functions in the pipeline.

Here's the example above, corrected to use `indent` to indent the `mychart_app` template correctly:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: {{ .Release.Name }}-configmap
  labels:
{{ include "mychart.app" . | indent 4 }}
data:
  myvalue: "Hello World"
  {{- range $key, $val := .Values.favorite }}
  {{ $key }}: {{ $val | quote }}
  {{- end }}
{{ include "mychart.app" . | indent 2 }}
```

Now the produced YAML is correctly indented for each section:

```yaml
# Source: mychart/templates/configmap.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: edgy-mole-configmap
  labels:
    app_name: mychart
    app_version: "0.1.0+1478129987"
data:
  myvalue: "Hello World"
  drink: "coffee"
  food: "pizza"
  app_name: mychart
  app_version: "0.1.0+1478129987"
```

> It is considered preferable to use `include` over `template` in Helm templates simply so that the output formatting can be handled better for YAML documents.

Sometimes we want to import content, but not as templates. That is, we want to import files verbatim. We can achieve this by accessing files through the `.Files` object described in the next section.
