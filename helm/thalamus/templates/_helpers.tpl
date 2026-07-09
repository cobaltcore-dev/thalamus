{{- define "thalamus.operator.name" -}}
{{- printf "%s-operator" .Release.Name }}
{{- end }}

{{- define "thalamus.operator.serviceAccountName" -}}
{{- default (include "thalamus.operator.name" .) .Values.operator.serviceAccount.name }}
{{- end }}

{{- define "thalamus.labels" -}}
app.kubernetes.io/part-of: thalamus
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/version: {{ .Chart.AppVersion }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{- define "thalamus.operator.selectorLabels" -}}
app.kubernetes.io/name: {{ include "thalamus.operator.name" . }}
app.kubernetes.io/component: operator
{{- end }}

{{- define "thalamus.model.selectorLabels" -}}
app.kubernetes.io/name: vllm-{{ .slug }}
app.kubernetes.io/component: model
{{- end }}

{{- define "thalamus.epp.selectorLabels" -}}
app.kubernetes.io/name: vllm-{{ .slug }}-epp
app.kubernetes.io/component: epp
{{- end }}

{{/* Render an image ref from {registry, repository, tag, digest} (bitnami-style: digest wins over tag). */}}
{{- define "thalamus.image" -}}
{{- $registry := .registry | default "" -}}
{{- $repository := required "image.repository is required" .repository -}}
{{- $tag := .tag | default "" -}}
{{- $digest := .digest | default "" -}}
{{- if $registry -}}{{ $registry }}/{{- end -}}
{{- $repository -}}
{{- if $digest -}}@{{ $digest }}
{{- else if $tag -}}:{{ $tag }}
{{- end -}}
{{- end }}
