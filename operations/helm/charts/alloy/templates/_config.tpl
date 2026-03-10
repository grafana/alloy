{{/*
Retrieve configMap name from the name of the chart or the ConfigMap the user
specified.
*/}}
{{- define "alloy.config-map.name" -}}
{{- $values := (mustMergeOverwrite .Values.alloy (or .Values.agent dict)) -}}
{{- if $values.configMap.name -}}
{{- $values.configMap.name }}
{{- else -}}
{{- include "alloy.fullname" . }}
{{- end }}
{{- end }}

{{/*
The name of the config file is the default or the key the user specified in the
ConfigMap.
*/}}
{{- define "alloy.config-map.key" -}}
{{- $values := (mustMergeOverwrite .Values.alloy (or .Values.agent dict)) -}}
{{- if $values.configMap.key -}}
{{- $values.configMap.key }}
{{- else -}}
config.alloy
{{- end }}
{{- end }}

{{/*
Retrieve Secret name from the name of the chart or the Secret the user
specified.
*/}}
{{- define "alloy.secret.name" -}}
{{- $values := (mustMergeOverwrite .Values.alloy (or .Values.agent dict)) -}}
{{- if $values.secret.name -}}
{{- $values.secret.name }}
{{- else -}}
{{- include "alloy.fullname" . }}
{{- end }}
{{- end }}

{{/*
The name of the config file is the default or the key the user specified in the
Secret.
*/}}
{{- define "alloy.secret.key" -}}
{{- $values := (mustMergeOverwrite .Values.alloy (or .Values.agent dict)) -}}
{{- if $values.secret.key -}}
{{- $values.secret.key }}
{{- else -}}
config.alloy
{{- end }}
{{- end }}

{{/*
Determine if using Secret for config (Secret takes precedence over ConfigMap).
*/}}
{{- define "alloy.config-source.use-secret" -}}
{{- $values := (mustMergeOverwrite .Values.alloy (or .Values.agent dict)) -}}
{{- if or $values.secret.create $values.secret.name -}}
true
{{- else -}}
false
{{- end }}
{{- end }}

{{/*
Get the config source name (Secret or ConfigMap).
*/}}
{{- define "alloy.config-source.name" -}}
{{- if eq (include "alloy.config-source.use-secret" .) "true" -}}
{{- include "alloy.secret.name" . }}
{{- else -}}
{{- include "alloy.config-map.name" . }}
{{- end }}
{{- end }}

{{/*
Get the config source key (Secret or ConfigMap).
*/}}
{{- define "alloy.config-source.key" -}}
{{- if eq (include "alloy.config-source.use-secret" .) "true" -}}
{{- include "alloy.secret.key" . }}
{{- else -}}
{{- include "alloy.config-map.key" . }}
{{- end }}
{{- end }}
