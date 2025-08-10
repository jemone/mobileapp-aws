{{- define "usersvc.name" -}}
usersvc
{{- end -}}

{{- define "usersvc.fullname" -}}
{{ include "usersvc.name" . }}-{{ .Release.Name }}
{{- end -}}
