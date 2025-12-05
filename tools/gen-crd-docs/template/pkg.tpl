{{ define "packages" }}

{{ with .packages}}
---
aliases:
- /docs/agent/latest/operator/crd/
- /docs/grafana-cloud/agent/operator/api/
- /docs/grafana-cloud/monitor-infrastructure/agent/operator/api/
- /docs/grafana-cloud/monitor-infrastructure/integrations/agent/operator/api/
- /docs/grafana-cloud/send-data/agent/operator/api/
canonical: https://grafana.com/docs/agent/latest/operator/api/
title: Custom Resource Definition Reference
description: Learn about the Grafana Agent API
weight: 500
---
# Custom Resource Definition Reference
{{ end}}

{{ range .packages }}
{{ with (index .GoPackages 0 )}}
{{ with .DocComments }}
{{ . }}
{{ end }}
{{ end }}

## Resource Types:
{{ range (visibleTypes (sortedTypes .Types)) }}
{{ if isExportedType . -}}
* [{{ typeDisplayName . }}]({{ linkForType . }})
{{- end }}
{{ end }}

{{ range (visibleTypes (sortedTypes .Types))}}
{{ template "type" . }}
{{ end }}
{{ end }}
{{ end }}
