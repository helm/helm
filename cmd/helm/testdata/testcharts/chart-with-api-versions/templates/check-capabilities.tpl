{{- if not (.Capabilities.APIVersions.Has "networking.k8s.io/v1") }}
  {{ fail "ERROR: APIVersion networking.k8s.io/v1 not found" }}
{{- end -}}