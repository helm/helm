{{/*
Simple helper for testing - returns the chart name
*/}}
{{- define "bigchart.name" -}}
{{- .Chart.Name }}
{{- end }}

{{/*
Simple helper for testing - returns the release name
*/}}
{{- define "bigchart.fullname" -}}
{{- .Release.Name }}
{{- end }}

