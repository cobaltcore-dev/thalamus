{{- define "thalamus.operator.name" -}}
{{- printf "%s-operator" .Release.Name }}
{{- end }}

{{- define "thalamus.operator.serviceAccountName" -}}
{{- default (include "thalamus.operator.name" .) .Values.operator.serviceAccount.name }}
{{- end }}

{{- define "thalamus.operator.selectorLabels" -}}
app.kubernetes.io/name: {{ include "thalamus.operator.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{- define "thalamus.operator.labels" -}}
{{ include "thalamus.operator.selectorLabels" . }}
app.kubernetes.io/version: {{ .Chart.AppVersion }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}
