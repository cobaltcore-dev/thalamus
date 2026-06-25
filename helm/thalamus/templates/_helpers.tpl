{{- define "thalamus.operator.selectorLabels" -}}
app.kubernetes.io/name: thalamus-operator
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{- define "thalamus.operator.labels" -}}
{{ include "thalamus.operator.selectorLabels" . }}
app.kubernetes.io/version: {{ .Chart.AppVersion }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}
