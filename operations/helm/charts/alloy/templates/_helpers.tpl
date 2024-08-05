{{/*
Expand the name of the chart.
*/}}
{{- define "alloy.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "alloy.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default .Chart.Name .Values.nameOverride }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "alloy.chart" -}}
{{- if index .Values "$chart_tests" }}
{{- printf "%s" .Chart.Name | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}

{{/*
Allow the release namespace to be overridden for multi-namespace deployments in combined charts
*/}}
{{- define "alloy.namespace" -}}
{{- if .Values.namespaceOverride }}
{{- .Values.namespaceOverride }}
{{- else }}
{{- .Release.Namespace }}
{{- end }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "alloy.labels" -}}
helm.sh/chart: {{ include "alloy.chart" . }}
{{ include "alloy.selectorLabels" . }}
{{- if index .Values "$chart_tests" }}
app.kubernetes.io/version: "vX.Y.Z"
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- else }}
{{/* substr trims delimeter prefix char from alloy.imageId output
    e.g. ':' for tags and '@' for digests.
    For digests, we crop the string to a 7-char (short) sha. */}}
app.kubernetes.io/version: {{ (include "alloy.imageId" .) | trunc 15 | trimPrefix "@sha256" | trimPrefix ":" | quote }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
app.kubernetes.io/part-of: alloy
{{- end }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "alloy.selectorLabels" -}}
app.kubernetes.io/name: {{ include "alloy.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "alloy.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "alloy.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Calculate name of image ID to use for "alloy.
*/}}
{{- define "alloy.imageId" -}}
{{- if .Values.image.digest }}
{{- $digest := .Values.image.digest }}
{{- if not (hasPrefix "sha256:" $digest) }}
{{- $digest = printf "sha256:%s" $digest }}
{{- end }}
{{- printf "@%s" $digest }}
{{- else if .Values.image.tag }}
{{- printf ":%s" .Values.image.tag }}
{{- else }}
{{- printf ":%s" .Chart.AppVersion }}
{{- end }}
{{- end }}

{{/*
Calculate name of image ID to use for "config-reloader".
*/}}
{{- define "config-reloader.imageId" -}}
{{- if .Values.configReloader.image.digest }}
{{- $digest := .Values.configReloader.image.digest }}
{{- if not (hasPrefix "sha256:" $digest) }}
{{- $digest = printf "sha256:%s" $digest }}
{{- end }}
{{- printf "@%s" $digest }}
{{- else if .Values.configReloader.image.tag }}
{{- printf ":%s" .Values.configReloader.image.tag }}
{{- else }}
{{- printf ":%s" "v0.8.0" }}
{{- end }}
{{- end }}

{{/*
Return the appropriate apiVersion for ingress.
*/}}
{{- define "alloy.ingress.apiVersion" -}}
{{- if and ($.Capabilities.APIVersions.Has "networking.k8s.io/v1") (semverCompare ">= 1.19-0" .Capabilities.KubeVersion.Version) }}
{{- print "networking.k8s.io/v1" }}
{{- else if $.Capabilities.APIVersions.Has "networking.k8s.io/v1beta1" }}
{{- print "networking.k8s.io/v1beta1" }}
{{- else }}
{{- print "extensions/v1beta1" }}
{{- end }}
{{- end }}

{{/*
Return if ingress is stable.
*/}}
{{- define "alloy.ingress.isStable" -}}
{{- eq (include "alloy.ingress.apiVersion" .) "networking.k8s.io/v1" }}
{{- end }}

{{/*
Return if ingress supports ingressClassName.
*/}}
{{- define "alloy.ingress.supportsIngressClassName" -}}
{{- or (eq (include "alloy.ingress.isStable" .) "true") (and (eq (include "alloy.ingress.apiVersion" .) "networking.k8s.io/v1beta1") (semverCompare ">= 1.18-0" .Capabilities.KubeVersion.Version)) }}
{{- end }}
{{/*
Return if ingress supports pathType.
*/}}
{{- define "alloy.ingress.supportsPathType" -}}
{{- or (eq (include "alloy.ingress.isStable" .) "true") (and (eq (include "alloy.ingress.apiVersion" .) "networking.k8s.io/v1beta1") (semverCompare ">= 1.18-0" .Capabilities.KubeVersion.Version)) }}
{{- end }}

{{/*
Return the appropriate apiVersion for PodDisruptionBudget.
*/}}
{{- define "alloy.controller.pdb.apiVersion" -}}
  {{- if and (.Capabilities.APIVersions.Has "policy/v1") (semverCompare ">=1.21-0" .Capabilities.KubeVersion.Version) -}}
    {{- print "policy/v1" -}}
  {{- else -}}
    {{- print "policy/v1beta1" -}}
  {{- end -}}
{{- end -}}
