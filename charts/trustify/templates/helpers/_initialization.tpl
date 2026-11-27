{{/* Initialization mode, shared by database and importer setup. Arguments: root context. */}}
{{- define "trustification.initialization.mode" -}}
{{- $mode := (.Values.initialization).mode | default "psql" -}}
{{- if not (has $mode (list "psql" "trustd")) -}}
{{- fail "initialization.mode must be psql or trustd" -}}
{{- end -}}
{{- $mode -}}
{{- end -}}

{{/*
Initialization image. Arguments: module context (root, module).
Legacy defaults are independent of the application image; explicit job overrides win.
*/}}
{{- define "trustification.initialization.image" -}}
{{- $defaults := deepCopy .root.Values.image -}}
{{- if eq (include "trustification.initialization.mode" .root) "psql" -}}
{{- $defaults = dict "registry" "docker.io/library" "name" "postgres" "version" "17.2" "pullPolicy" (.root.Values.image.pullPolicy | default "IfNotPresent") -}}
{{- end -}}
{{- $image := dict "root" .root "image" (.module.image | default dict) "defaults" $defaults -}}
image: {{ include "trustification.common.imageName" $image | quote }}
imagePullPolicy: {{ include "trustification.common.imagePullPolicy" $image | quote }}
{{- end -}}
