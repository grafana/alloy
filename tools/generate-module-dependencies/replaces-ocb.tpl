  # BEGIN GENERATED REPLACES - DO NOT EDIT MANUALLY
{{- range . }}
{{- if .Comment }}
  # {{ .Comment }}
{{- end }}
  - {{ .Dependency }} => {{ .Replacement }}
{{- end }}
  # END GENERATED REPLACES