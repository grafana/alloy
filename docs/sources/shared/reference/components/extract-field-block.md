---
aliases:
- /docs/agent/shared/flow/reference/components/extract-field-block/
- /docs/grafana-cloud/agent/shared/flow/reference/components/extract-field-block/
- /docs/grafana-cloud/monitor-infrastructure/agent/shared/flow/reference/components/extract-field-block/
- /docs/grafana-cloud/monitor-infrastructure/integrations/agent/shared/flow/reference/components/extract-field-block/
- /docs/grafana-cloud/send-data/agent/shared/flow/reference/components/extract-field-block/
canonical: https://grafana.com/docs/agent/latest/shared/flow/reference/components/extract-field-block/
description: Shared content, extract field block
headless: true
---

The following attributes are supported:

Name        | Type     | Description                                                                            | Default | Required
------------|----------|----------------------------------------------------------------------------------------|---------|---------
`from`      | `string` | The source of the labels or annotations. Allowed values are `pod` and `namespace`.     | `pod`   | no
`key_regex` | `string` | A regular expression used to extract a key that matches the regular expression.        | `""`    | no
`key`       | `string` | The annotation or label name. This key must exactly match an annotation or label name. | `""`    | no
`regex`     | `string` | An optional field used to extract a sub-string from a complex field value.             | `""`    | no
`tag_name`  | `string` | The name of the resource attribute added to logs, metrics, or spans.                   | `""`    | no

When you don't specify the `tag_name`, a default tag name is used with the format:
* `k8s.pod.annotations.<annotation key>`
* `k8s.pod.labels.<label key>`

For example, if `tag_name` isn't specified and the key is `git_sha`, the attribute name will be `k8s.pod.annotations.git_sha`.

You can set either the `key` attribute or the `key_regex` attribute, but not both.
When `key_regex` is present, `tag_name` supports back reference to both named capturing and positioned capturing.

For example, assume your pod spec contains the following labels:
* `app.kubernetes.io/component: mysql`
* `app.kubernetes.io/version: 5.7.21`

If you'd like to add tags for all labels with the prefix `app.kubernetes.io/` and trim the prefix, then you can specify the following extraction rules:

```river
extract {
	label {
	    from = "pod"
		key_regex = "kubernetes.io/(.*)"
		tag_name  = "$1"
	}
}
```

These rules add the `component` and `version` tags to the spans or metrics.

You can set the `from` attribute to either `"pod"` or `"namespace"`.
