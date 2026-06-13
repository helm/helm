{{- /*
common.labelize takes a dict or map and generates labels.

Values will be quoted. Keys will not.

Example output:

  first: "Matt"
  last: "Butcher"

*/ -}}
{{- define "common.labelize" -}}
{{- range $k, $v := . }}
{{ $k }}: {{ $v | quote }}
{{- end -}}
{{- end -}}

{{- /*
common.labels.standard prints the standard Helm labels.

The standard labels are frequently used in metadata.
*/ -}}
{{- define "common.labels.standard" -}}
app.kubernetes.io/name: {{ template "common.name" . }}
helm.sh/chart: {{ template "common.chartref" . }}
app.kubernetes.io/managed-by: {{ .Release.Service | quote }}
app.kubernetes.io/instance: {{ .Release.Name | quote }}
{{- end -}}
