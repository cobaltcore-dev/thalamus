{{/* Render an image ref from {registry, repository, tag, digest} (bitnami-style: digest wins over tag). */}}
{{- define "model-server.image" -}}
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