{{/* vim: set filetype=mustache: */}}
{{/*
Expand the name of the chart.
*/}}
{{define "name"}}{{default "nginx" .Values.nameOverride | trunc 54 | trimSuffix "-" }}{{end}}

{{/*
Create a default fully qualified app name.

We truncate at 54 chars because some Kubernetes name fields are limited to this
(by the DNS naming spec).
*/}}
{{define "fullname"}}
{{- $name := default "nginx" .Values.nameOverride -}}
{{printf "%s-%s" .Release.Name $name | trunc 54 | trimSuffix "-" -}}
{{end}}
