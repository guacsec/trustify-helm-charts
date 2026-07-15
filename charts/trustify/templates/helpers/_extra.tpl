{{/*
Additional volumes

Arguments (dict):
  * root - .
*/}}
{{- define "trustification.application.extraVolumes" }}
{{- with .root.Values.extraVolumes }}
{{- . | toYaml }}
{{- end }}
{{- end }}

{{/*
Additional volume mounts

Arguments (dict):
  * root - .
*/}}
{{- define "trustification.application.extraVolumeMounts" }}
{{- with .root.Values.extraVolumeMounts }}
{{- . | toYaml }}
{{- end }}
{{- end }}

{{/*
Additional environment variables

Arguments (dict):
  * root - .
  * module - module object
*/}}
{{- define "trustification.application.extraEnv" }}
{{- with .module.extraEnv }}
{{- . | toYaml }}
{{- end }}
{{- with .root.Values.extraEnv }}
{{- . | toYaml }}
{{- end }}
{{- end }}
