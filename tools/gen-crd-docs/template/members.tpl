{{ define "members" }}

{{ range .Members }}
{{ if not (hiddenMember .)}}
|`{{ fieldName . }}`<br/>_{{ if linkForType .Type }}[{{ typeDisplayName .Type }}]({{ linkForType .Type}}){{ else }}{{ typeDisplayName .Type }}{{ end }}_|{{ if fieldEmbedded . }}
(Members of `{{ fieldName . }}` are embedded into this type.){{ end}} {{ if isOptionalMember .}} _(Optional)_ {{ end }} {{ range .CommentLines }}{{ . }} {{ end }} {{ if and (eq (.Type.Name.Name) "ObjectMeta") }} Refer to the Kubernetes API documentation for the fields of the `metadata` field. {{ end }}|

{{ if or (eq (fieldName .) "spec") }}
{{ template "members" .Type }}
{{ end }}
{{ end }}
{{ end }}

{{ end }}
