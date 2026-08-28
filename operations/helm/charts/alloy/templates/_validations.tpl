{{/*
Validations — these are evaluated at render time via templates/validations.yaml.
Each validation should `fail` with an instructive, multi-line message that names
the offending values and tells the user what to set.
*/}}

{{- define "alloy.validations.externalHPA" -}}
{{- if and .Values.controller.autoscaling.horizontal.enabled .Values.controller.autoscaling.horizontal.externalHPA -}}
{{- $msg := printf "controller.autoscaling.horizontal.enabled and controller.autoscaling.horizontal.externalHPA are mutually exclusive.\nSet exactly one:\n  - horizontal.enabled: true       chart manages an HPA based on CPU/memory utilization.\n  - horizontal.externalHPA: true   omit replicas and skip the chart's HPA so an external controller (e.g. KEDA) can own scaling." -}}
{{- fail $msg -}}
{{- end -}}
{{- end -}}

{{- define "alloy.validations.all" -}}
{{- include "alloy.validations.externalHPA" . -}}
{{- end -}}
