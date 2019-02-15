{{- /*
common.chartref prints a chart name and version.

It does minimal escaping for use in Kubernetes labels.

Example output:

  zookeeper-1.2.3
  wordpress-3.2.1_20170219

*/ -}}
{{- define "common.chartref" -}}
  {{- replace "+" "_" .Chart.Version | printf "%s-%s" .Chart.Name -}}
{{- end -}}
