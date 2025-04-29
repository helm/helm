{{- /*
common.metadata creates a standard metadata header.
It creates a 'metadata:' section with name and labels.
*/ -}}
{{ define "common.metadata" -}}
metadata:
  name: {{ template "common.fullname" . }}
  labels:
{{ include "common.labels.standard" . | indent 4 -}}
{{- end -}}
