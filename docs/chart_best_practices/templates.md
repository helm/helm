# Templates

This part of the Best Practices Guide focuses on templates.

## Structure of templates/

The templates directory should be structured as follows:

- Template files should have the extension `.yaml` if they produce YAML output. The
  extension `.tpl` may be used for template files that produce no formatted content.
- Template file names should use dashed notation (`my-example-configmap.yaml`), not camelcase.
- Each resource definition should be in its own template file.
- Template file names should reflect the resource kind in the name. e.g. `foo-pod.yaml`,
  `bar-svc.yaml`

## Names of Defined Templates

Defined templates (templates created inside a `{{ define }} ` directive) are
globally accessible. That means that a chart and all of its subcharts will have
access to all of the templates created with `{{ define }}`.

For that reason, _all defined template names should be namespaced._

Correct:

```yaml
{{- define "nginx.fullname" }}
{{/* ... */}}
{{ end -}}
```

Incorrect:

```yaml
{{- define "fullname" -}}
{{/* ... */}}
{{ end -}}
```
It is highly recommended that new charts are created via `helm create` command as the template names are automatically defined as per this best practice.

## Formatting Templates

Templates should be indented using _two spaces_ (never tabs).

Template directives should have whitespace after the opening  braces and before the
closing braces:

Correct:
```
{{ .foo }}
{{ print "foo" }}
{{- print "bar" -}}
```

Incorrect:
```
{{.foo}}
{{print "foo"}}
{{-print "bar"-}}
```

Templates should chomp whitespace where possible:

```
foo:
  {{- range .Values.items }}
  {{ . }}
  {{ end -}}
```

Blocks (such as control structures) may be indented to indicate flow of the template code.

```
{{ if $foo -}}
  {{- with .Bar }}Hello{{ end -}}
{{- end -}} 
```

However, since YAML is a whitespace-oriented language, it is often not possible for code indentation to follow that convention.

## Whitespace in Generated Templates

It is preferable to keep the amount of whitespace in generated templates to
a minimum. In particular, numerous blank lines should not appear adjacent to each
other. But occasional empty lines (particularly between logical sections) is
fine.

This is best:

```yaml
apiVersion: batch/v1
kind: Job
metadata:
  name: example
  labels:
    first: first
    second: second
```

This is okay:

```yaml
apiVersion: batch/v1
kind: Job

metadata:
  name: example

  labels:
    first: first
    second: second

```

But this should be avoided:

```yaml
apiVersion: batch/v1
kind: Job

metadata:
  name: example





  labels:
    first: first

    second: second

```

## Comments (YAML Comments vs. Template Comments)

Both YAML and Helm Templates have comment markers.

YAML comments:
```yaml
# This is a comment
type: sprocket
```

Template Comments:
```yaml
{{- /*
This is a comment.
*/ -}}
type: frobnitz
```

Template comments should be used when documenting features of a template, such as explaining a defined template:

```yaml
{{- /*
mychart.shortname provides a 6 char truncated version of the release name.
*/ }}
{{ define "mychart.shortname" -}}
{{ .Release.Name | trunc 6 }}
{{- end -}}

```

Inside of templates, YAML comments may be used when it is useful for Helm users to (possibly) see the comments during debugging.

```
# This may cause problems if the value is more than 100Gi
memory: {{ .Values.maxMem | quote }}
```

The comment above is visible when the user runs `helm install --debug`, while
comments specified in `{{- /* */ -}}` sections are not.

## Use of JSON in Templates and Template Output

YAML is a superset of JSON. In some cases, using a JSON syntax can be more
readable than other YAML representations.

For example, this YAML is closer to the normal YAML method of expressing lists:

```yaml
arguments: 
  - "--dirname"
  - "/foo"
```

But it is easier to read when collapsed into a JSON list style:

```yaml
arguments: ["--dirname", "/foo"]
```

Using JSON for increased legibility is good. However, JSON syntax should not
be used for representing more complex constructs.

When dealing with pure JSON embedded inside of YAML (such as init container
configuration), it is of course appropriate to use the JSON format.
