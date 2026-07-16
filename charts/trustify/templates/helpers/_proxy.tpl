{{/*
Proxy env-vars for containers that need HTTP proxy support.

Arguments (dict):
  * root - .
*/}}
{{- define "trustification.application.proxy.envVars" -}}
{{- with .root.Values.proxy }}
{{- with .httpProxy }}
- name: HTTP_PROXY
  value: {{ . | quote }}
{{- end }}
{{- with .httpsProxy }}
- name: HTTPS_PROXY
  value: {{ . | quote }}
{{- end }}
{{- with .noProxy }}
- name: NO_PROXY
  value: {{ . | quote }}
{{- end }}
{{- end }}
{{- end }}
