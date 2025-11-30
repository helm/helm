{{/*
Generate the name of the chart, including release name, namespace, and app name
*/}}
{{- define "otlp_forwarder.fullname" -}}
{{- .Release.Name }}-{{ .Chart.Name }}
{{- end -}}

{{/*
Common labels for the chart
*/}}
{{- define "otlp_forwarder.labels" -}}
app.kubernetes.io/name: {{ .Chart.Name }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/version: {{ .Chart.AppVersion }}
app.kubernetes.io/component: {{ .Values.component | default "otlp-forwarder" }}
app.kubernetes.io/part-of: {{ .Values.partOf | default "otel" }}
{{- end -}}

{{/*
Selector labels for the chart
*/}}
{{- define "otlp_forwarder.selectorLabels" -}}
app.kubernetes.io/name: {{ .Chart.Name }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end -}}

{{/*
Name of the ServiceAccount
*/}}
{{- define "otlp_forwarder.serviceAccountName" -}}
{{ .Release.Name }}-serviceaccount
{{- end -}}
