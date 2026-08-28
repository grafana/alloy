# Write documentation for Alloy components

This guide outlines best practices for writing reference documentation for Alloy
components.

Component reference documentation is stored in [docs/sources/reference/components][docs source],
and published to the [Grafana documentation website][hosted docs].

Documentation for a component should follow best practices as much as possible
so that things are kept consistent. Exceptions can be made when needed. Refer to the
list of [Exceptions][] at the bottom of this page for examples of exceptions.

[docs source]: ../sources/alloy/reference/components
[hosted docs]: https://grafana.com/docs/alloy/latest/reference/components
[Exceptions]: #exceptions

## General guidelines

* Follow the [Grafana Writers' Toolkit](https://grafana.com/docs/writers-toolkit/).
* Follow the [Google developer documentation style guide](https://developers.google.com/style).
* Prefer being explicit over making assumptions; assume that your documentation
  may be the first page someone visits.
* Aim for component documentation to be consistent with documentation for other
  components.
* Refer to the component by its full name as much as possible rather than "the
  component."

## Page structure

Component reference pages should start with YAML front matter containing the
canonical path, a short description, and the name of the component as the page title:

```markdown
---
canonical: https://grafana.com/docs/alloy/latest/reference/components/PATH_TO_MARKDOWN_FILE/
description: SHORT DESCRIPTION OF THE COMPONENT
title: COMPONENT_NAME
---
```

If documenting an experimental component, include the following inside the
front matter:

```markdown
labels:
  stage: experimental
  products:
    - oss
```

If documenting a public preview component, include the following inside the
front matter:

```markdown
labels:
  stage: public-preview
  products:
    - oss
```

If documenting an generally available component, include the following inside the
front matter:

```markdown
labels:
  stage: general-availability
  products:
    - oss
```

All component reference pages should always be broken down into the following
sections:

1. Title
2. Usage
3. Arguments
4. Blocks
5. Exported fields
6. Component health
7. Debug information
8. Debug metrics
9. Examples

If a section doesn't apply to a component (such as a component not exposing
any debug information), the section should still be included, with a sentence
explicitly documenting that it doesn't apply to a component. For example:

```markdown
## Debug information

`COMPONENT_NAME` doesn't expose any component-specific debug information.
```

Always including the headers removes as much guesswork as possible from
readers, so they know for certain that there is no debug information, rather
than assuming it must not exist if it's not documented.

### `Title`

The Title section is an `h1` section which provides a brief description of the
component. The header should be named after the component. For example:

```markdown
# `local.file`

`local.file` exposes the contents of a file on disk to other components.

The most common use of `local.file` is to load secrets (e.g., API keys) from
files.
```

Use backticks when referring to the component in the body of the Title section,
and in the header.

The Title section should be kept high-level and as small as possible. Detailed
information on component behavior should be kept to [Arguments](#arguments) and
[Blocks](#blocks) sections as appropriate.

If documenting an experimental component, include the following:

```markdown
{{< docs/shared lookup="stability/experimental.md" source="alloy" version="<ALLOY_VERSION>" >}}
```

If documenting a public preview component, include the following:

```markdown
{{< docs/shared lookup="stability/public_preview.md" source="alloy" version="<ALLOY_VERSION>" >}}
```

If your component supports labels, add the following as the last paragraph of
the Title section:

```markdown
You can specify multiple `COMPONENT_NAME` components by giving them different labels.
```

### Usage

The Usage section provides a minimal example containing _required_ attributes
and blocks to configure a component.

It starts with an `h2` header called Usage.

The Usage section should be composed of a single Alloy code block, with no
description. Use `<YELLING_SNAKE_CASE>` to refer to values the user must replace.
For example:

````markdown
## Usage

```alloy
pyroscope.scrape "<LABEL>" {
  targets    = <TARGET_LIST>
  forward_to = <RECEIVER_LIST>
}
```
````

### Arguments

The Arguments section details the set of arguments the component supports,
followed by a detailed descriptions of how the arguments modify the component
behavior. The section starts with an `h2` header called Arguments.

If the component doesn't support any arguments, and is only configured through
blocks, the content of the section should be the following paragraph:

```markdown
The `COMPONENT_NAME` component doesn't support any arguments. You can configure this component with blocks.
```

When arguments are supported by the component, the set of arguments should be
listed using a Markdown table, with the following columns:

| Column      | Description                       |
| ----------- | --------------------------------- |
| Name        | Argument name.                    |
| Type        | Argument type.                    |
| Description | Argument description.             |
| Default     | Default value for the argument.   |
| Required    | Whether the argument is required. |

If possible, sort the table rows alphabetically, required first, then optional.

A paragraph with the content "You can use the following arguments with `COMPONENT_NAME`:" should
always precede the arguments table.

For example:

```markdown
You can use the following arguments with `COMPONENT_NAME`:

Name             | Type       | Description                            | Default      | Required |
---------------- | ---------- | -------------------------------------- | ------------ | -------- |
`filename`       | `string`   | Path of the file on disk to watch.     |              | yes      |
`detector`       | `string`   | Which file change detector to use.     | `"fsnotify"` | no       |
`is_secret`      | `bool`     | Marks the file as containing a secret. | `false`      | no       |
`poll_frequency` | `duration` | How often to poll for file changes.    | `"1m"`       | no       |
```

Values for the Name column should be the backticked argument name, such as `` `targets` ``.

Values for the Type column should be the backticked Alloy syntax type (for example, `string`, `bool`, `number`).
Arrays should be represented as `list(INNER_TYPE)`, and dictionaries should be represented as `map(VALUE_TYPE)`.

In addition to the known types, the following are excepted as convention:

* `duration`: A string which is parsed as a duration.
* `secret`: A string which is treated as sensitive.
* `capsule(T)`: An internal encapsulated value.

Values for the Description column should be kept as short as possible to keep
the table small and readable. Detailed descriptions should be placed outside of
the table. Use full sentences for argument descriptions, ending them in
periods.

Values for the Default column may be omitted if the default is the zero value
for the given Alloy syntax type. The Default column must be empty for required
attributes.

Values for the Required column must be either "yes" (for required attributes)
or "no" (for optional attributes).

The set of documented attributes should only contain attributes in the root
block of the component. The list of supported blocks and the list of attributes
for those blocks are documented in other sections.

A detailed description of component behavior and how arguments affect that
component behavior should follow the arguments table. To refer to an argument,
use ``The `ARGUMENT_NAME` argument`` to make it extremely clear what you're
referring to.

Descriptions for component behavior should be limited to what's directly
relevant to the arguments. If there is component behavior relevant to a
specific block, describe that component behavior in the documentation section
for that block instead.

It's encouraged to provide configuration snippets for the arguments if it aids
documentation.

### Blocks

The Blocks section details the hierarchy of blocks the component supports. The
section starts with an `h2` header called Blocks.

If the component doesn't support any blocks, the content of the section should
be the following paragraph:

```markdown
The `COMPONENT_NAME` component does not support any blocks. You can configure this component with arguments.
```

When blocks are supported by the component, the set of blocks should be listed
using a Markdown table, with the following columns:

| Column      | Description                       |
| ----------- | --------------------------------- |
| Hierarchy   | Block path.                       |
| Block       | Link to block documentation.      |
| Description | Block description.                |
| Required    | Whether the block is required.    |

If possible, sort the table rows alphabetically, required first, then optional.

A paragraph with the content "You can use the following blocks with `COMPONENT_NAME`:" should
always precede the blocks table.

For example:

```markdown
You can use the following blocks with `COMPONENT_NAME`:

| Block                                            | Description                        | Required |
| ------------------------------------------------ | ---------------------------------- | -------- |
| [`client`][client]                               | HTTP client settings.              | no       |
| `client` > [`basic_auth`][basic_auth]            | Basic authentication settings.     | no       |
| `client` > [`authorization`][authorization]      | Generic authentication settings.   | no       |
| `client` > [`oauth2`][oauth2]                    | OAuth 2.0 authentication settings. | no       |
| `client` > `oauth2` > [`tls_config`][tls_config] | TLS settings for OAuth 2.0.        | no       |
| `client` > [`tls_config`][tls_config]            | TLS settings the HTTP client.      | no       |
```

Use the > character to represent nested blocks.

Values for the Block column should contain a link to the header in the same
documentation page which describes the block. **Don't** link to another
component page that happens to have the same block. Prefer re-documenting
blocks and being explicit instead of having users jump around. Use the
[`docs/shared` shortcode][docs-shared] to deduplicate block definitions across
multiple components.

Values for the Description column should be kept as short as possible to keep
the table small and readable. Use full sentences for block descriptions,
ending them in periods.

Values for the Required column should be the text "yes" (for required blocks)
or "no" (for optional blocks).

If nested blocks are used in the blocks table (like `client` > `basic_auth` in
the example), a description of what the > symbol means should be included in
the following paragraph after the block table:

```markdown
The `>` symbol indicates deeper levels of nesting.
For example, `PARENT_BLOCK` > `CHILD_BLOCK` refers to a `CHILD_BLOCK` block defined inside a `PARENT_BLOCK` block.
```

When including this paragraph, replace `PARENT_BLOCK` and `CHILD_BLOCK` with
block names from your component to provide a concrete example of what the
hierarchy represents.

A set of sub-sections for each defined block should follow the Blocks section.
Each unique block type will only have one Block section, regardless of how many
times that block type appears in the blocks table. Refer to [Block section](#block-section)
for a description on these sections.

[docs-shared]: https://grafana.com/docs/writers-toolkit/writing-guide/reuse-shared-content/

#### Block section

The Block section is a sub-section of Blocks describing an individual block
supported by a component. There is one Block section per recognized block type
within the component.

The Block section starts with an `h3` header with the name of the block.
Use backticks around the name of the block in the header.

Block sections are similar to Arguments section, where it's composed of:

1. A brief description of the block.
2. A table of arguments supported by the block.
3. Detailed description for how that block impacts component behavior, and how
   the block's arguments can be used to modify that behavior.

Where possible, document the block sections in the same order as given in the blocks table.

Refer to [Arguments](#arguments) for a description of how to write the arguments
table and block-level descriptions following that table.

For example:

```markdown
### `tls`

The `tls` block configures TLS settings used when connecting to the server. If
the `tls` block isn't provided, connections to the server are unencrypted.

The following arguments are supported:

Name              | Type       | Description                                                   | Default | Required |
----------------- | ---------- | ------------------------------------------------------------- | ------- | -------- |
`ca_file`         | `string`   | Path to the CA file.                                          |         | no       |
`cert_file`       | `string`   | Path to the TLS certificate.                                  |         | no       |
`client_ca_file`  | `string`   | Path to the CA file used to authenticate client certificates. |         | no       |
`key_file`        | `string`   | Path to the TLS certificate key.                              |         | no       |
`max_version`     | `string`   | Maximum acceptable TLS version for connections.               |         | no       |
`min_version`     | `string`   | Minimum acceptable TLS version for connections.               |         | no       |
`reload_interval` | `duration` | Frequency to reload the certificates.                         |         | no       |

Default values for the `min_version` and `max_version` arguments are inherited
from Go, currently TLS 1.2 and TLS 1.3 respectively. When these arguments are
not provided, their Go-inherited defaults will not display in the component UI
page.
```

It's encouraged for block sections to provide configuration snippets for the
block if it aids documentation.

### Exported fields

The Exported fields section details a list of fields which the component
exports. The section starts with an `h2` header called Exported fields.

If the component doesn't export any fields, the content of the section should
be the following paragraph:

```markdown
The `COMPONENT_NAME` component doesn't export any values.
```

When the component exports values, it should provide a table of exported values
with the following columns:

| Column      | Description                          |
| ----------- | ------------------------------------ |
| Name        | Name of exported value.              |
| Type        | Alloy syntax type of exported value. |
| Description | Description of exported value.       |

Values for the Name column should be the backticked exported field name, such
as `` `targets` ``.

Values for the Type column should be the backticked Alloy syntax type. These types
should follow the same guidelines detailed in the Type column in the [Arguments
block](#arguments).

Values for the description column should be kept as short as possible to keep
the table small and readable.

A paragraph with the content "The following fields are exported and can be
referenced by other components:" should always prefix the exported fields
table.

Following the exported fields table, a longer description of each exported
fields may be provided, but this normally isn't done.

### Component health

The Component health section describes when the component is healthy. The
section starts with an `h2` header called Component health.

If the component doesn't have special health logic, the content of the section
should be the following paragraph:

```markdown
`COMPONENT_NAME` is only reported as unhealthy if given an invalid configuration.
```

Otherwise, write a detailed description for when the component is reported as
healthy.

### Debug information

The Debug information section describes debug information exposed in the
Grafana Alloy UI. The section starts with an `h2` header called Debug
information.

If the component doesn't expose any debug information, the content of the
section should be the following paragraph:

```markdown
`COMPONENT_NAME` doesn't expose any component-specific debug information.
```

Otherwise, write a high-level description for what debug information the
component provides. Don't document the attributes or blocks which are exposed
through the debug information.

### Debug metrics

The Debug metrics section describes what Prometheus metrics are exposed by a
component. The section starts with an `h2` header called Debug metrics.

If the component doesn't expose any debug metrics, the content of the section
should be the following paragraph:

```markdown
`COMPONENT_NAME` doesn't expose any component-specific debug metrics.
```

When the component exports values, it should provide a table of exposed metrics
with the following columns:

| Column      | Description                       |
| ----------- | --------------------------------- |
| Name        | Name of Prometheus metric.        |
| Type        | Prometheus metric type.           |
| Description | Metric description.               |

Values in the Name and Type column should be backticked.

Values in the Type column should be a Prometheus metric type, one of `counter`,
`gauge`, `histogram`, `native histogram`, or `summary`.

A paragraph with the content "The following Prometheus metrics are exposed:"
should always prefix the metrics table.

### Examples

The Examples section provides copy-and-paste Alloy pipelines which use the
component. The section starts with an `h2` header called Examples. If there is
only one example, call the section Example instead.

If there is more than one example, each example should have an `h3` header
containing a descriptive name. For example:

```markdown
## Examples

### Send metrics to a Prometheus server
...

### Send metrics to a Prometheus server using Oauth2 authentication
...
```

The Examples section should be composed of a brief description of each example,
followed by the example in a code block. For example:

````markdown
This example reads a JSON array of objects from an endpoint and uses them for
the set of scrape targets:

```alloy
remote.http "targets" {
  url = <TARGETS_URL>
}

prometheus.scrape "default" {
  targets    = encoding.from_json(remote.http.targets.content)
  forward_to = [prometheus.remote_write.default.receiver]
}

prometheus.remote_write "default" {
  client {
    url = <PROMETHEUS_URL>
  }
}
```
Replace the following:
  - `<TARGETS_URL>`: The URL to fetch the JSON array of objects from.
  - `<PROMETHEUS_URL>`: The URL of the Prometheus remote_write-compatible server to send metrics to.
````

Each example should be a full pipeline when possible, rather than just the
individual component being documented.

Placeholder variables should follow the [Writer's Toolkit](https://grafana.com/docs/writers-toolkit/write/style-guide/write-for-developers/#placeholder-variables) guidelines, where:

* Placeholder variables in code blocks are written as `` `<PLACEHOLDER>` ``
* Placeholder variables in the text are written as ``_`<PLACEHOLDER>`_``

Examples of the new component should avoid using placeholder variables and instead use
realistic example values. For example, if documenting a `prometheus.scrape` component, use:

  ```grafana-alloy
  remote.http "targets" {
    url = "http://localhost:8080/targets"
  }
  ```

If an example includes a placeholder variable, make sure to include a brief description
of what the placeholder variable is. For example:

```markdown
Replace _`<API_URL>`_ with the URL of the API to query.
```

or if there are multiple placeholders:

````markdown
Replace the following:
* _`<API_URL>`_: The URL of the API to query.
* _`<API_KEY>`_: The API key to use when querying the API.
````

If an example includes clarifying comments, make sure that the relevant
Arguments or block header includes sufficient explanation to be the official
source for the clarifying comment. Clarifying comments must only be used be
supplementary information to reinforce knowledge, and not as the primary source
of information.

Examples should be formatted using the [`alloy fmt`](https://grafana.com/docs/alloy/latest/reference/cli/fmt/) command.

## Exceptions

The rules described in this page should be sufficient in most cases for being
able to write detailed documentation.

However, there have been some cases where the page structure needed to be
changed to properly document a component. The following sections describe
some instances where exceptions had to be made.

### `loki.source.podlogs`

The [`loki.source.podlogs`][loki.source.podlogs] component documentation needed to add an extra
section to document the PodLogs CRD, since we don't yet have a way of
documenting auxiliary artifacts which are related to a component.

### `otelcol.processor.transform`

The [`otelcol.processor.transform`][otelcol.processor.transform] component documentation needed to add
an extra section about [OTTL Contexts][ottl context] because there is no appropriate OTEL docs page
that we could link to.

[loki.source.podlogs]: ../sources/reference/components/loki.source.podlogs.md
[otelcol.processor.transform]: ../sources/reference/components/otelcol.processor.transform.md
[ottl context]: https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/main/pkg/ottl/contexts/README.md
