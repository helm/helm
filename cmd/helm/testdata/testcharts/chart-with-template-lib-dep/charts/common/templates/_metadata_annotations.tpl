{{- /*
common.hook defines a hook.

This is to be used in a 'metadata.annotations' section.

This should be called as 'template "common.metadata.hook" "post-install"'

Any valid hook may be passed in. Separate multiple hooks with a ",".
*/ -}}
{{- define "common.hook" -}}
"helm.sh/hook": {{printf "%s" . | quote}}
{{- end -}}

{{- define "common.annote" -}}
{{- range $k, $v := . }}
{{ $k | quote }}: {{ $v | quote }}
{{- end -}}
{{- end -}}
