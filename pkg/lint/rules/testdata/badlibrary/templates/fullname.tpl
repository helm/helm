{{- define "common.fullname"}}
  {{- $global := default (dict) .Values.global -}}
  {{- $base := default .Chart.Name .Values.nameOverride -}}
  {{- $gpre := default "" $global.namePrefix -}}
  {{- $pre := default "" .Values.namePrefix -}}
  {{- $suf := default "" .Values.nameSuffix -}}
  {{- $gsuf := default "" $global.nameSuffix -}}
  {{- $fullname := print $gpre $pre $base $suf $gsuf -}}
  {{- $fullname | lower | trunc 54 | trimSuffix "-" -}}
{{- end -}}
