// BEGIN GENERATED REPLACES - DO NOT EDIT MANUALLY
{{- range . }}
{{- if .Comment }}
// {{ .Comment }}
{{- end }}
replace {{ .Dependency }} => {{ .Replacement }}
{{ end -}}
// END GENERATED REPLACES

