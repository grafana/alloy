# Static tests

A static test checks an Alloy configuration against rules, without running
it. It takes milliseconds, so it's much cheaper than a
[pipeline test](../pipelinetest/README.md).

`alloy test static` is **experimental**: the command, the file format and the
rule types can change in any release. For the design, refer to
[DECISIONS.md](DECISIONS.md).

## Run tests

```sh
alloy test static                # every *.statictest.yaml under the current directory
alloy test static ./config.statictest.yaml
alloy test static --output=json  # one JSON report, for CI or an agent
alloy test                       # static and pipeline tests
```

`alloy test static` finds files the same way as `alloy test pipeline`. For
`--match` and the other flags, run `alloy test static --help`.

A failed rule shows its message, its description and its docs link. Read
them to decide whether your config is a valid exception. To accept one,
change or remove the rule. A failed `severity: should` rule shows `!`, and
doesn't fail the test.

## File format

```yaml
kind: statictest
version: 1
config:
  path: config.alloy     # relative to the test file

rules:
  - name: clustering on all Prometheus scrape components
    description: >-
      If one Prometheus scrape component has no clustering block, each
      instance scrapes all of its targets, and nothing reports an error.
    docs: https://grafana.com/docs/alloy/latest/get-started/clustering/
    severity: must       # must (default) or should
    type: all-or-none
    select: [prometheus.scrape, prometheus.operator.podmonitors, prometheus.operator.servicemonitors]
    path: clustering.enabled
    equals: true

  - name: logs go only to our gateway
    description: Our security policy doesn't permit other log destinations.
    type: attribute
    select: loki.write
    path: endpoint[*].url
    matches: 'https://logs-gateway\.example\.com/.*'
```

Unknown fields fail the test. Several test files can point at the same
config.

### Select components

`select` takes one pattern or a list. A pattern matches a component name,
such as `prometheus.scrape`, or a component ID, such as
`prometheus.scrape.default`. `*` matches any characters, so `loki.source.*`
matches every log source, and `*` matches every component.

Only top-level components are checked. Components inside modules aren't.

### Paths

`path` is a dot-separated path inside a component:

- `clustering.enabled` is the `enabled` attribute of the `clustering` block.
- `endpoint.url` and `endpoint[*].url` are the `url` of every `endpoint`
  block. `endpoint[0].url` is the first one.
- Block names with dots work, such as `stage.static_labels.values` in
  `loki.process`.

A value must be a constant to be checked. An attribute that calls a
function, such as `sys.env`, or reads another component's export, fails the
rule. The checker doesn't know the value it has when Alloy runs.

## Rule types

| Type            | Passes when                                                                   | Fields                                   |
| --------------- | ----------------------------------------------------------------------------- | ---------------------------------------- |
| `attribute`     | every selected component has the path, and every value meets the condition    | `select`, `path`, `equals` or `matches`  |
| `all-or-none`   | all selected components meet the condition, or none of them do                | `select`, `path`, `equals` or `matches`  |
| `count`         | the number of selected components, or of values at `path`, is in range        | `select`, `path` (optional), `min`, `max` |
| `flows-through` | every path of data from a `from` component to a `to` component passes `through` | `from`, `to`, `through`                |

- `equals` compares with a YAML value: a string, number, bool, list or map.
- `matches` is a regular expression that must match the whole string.
- A rule that selects no components passes, with a note.
- `flows-through` follows references between components. A reference to a
  `receiver` or `input` export, such as in `forward_to`, sends data to the
  referenced component. A reference to another export, such as `targets`,
  sends data from it.

Any rule can have a `when` condition, which is a count. The rule only
applies when the condition holds:

```yaml
- name: no clustering for node logs
  type: count
  select: "*"
  path: clustering
  max: 0
  when:                  # only when the config scrapes no metrics
    select: [prometheus.scrape, prometheus.operator.podmonitors, prometheus.operator.servicemonitors]
    max: 0
```

## Not supported yet

- OTel Collector configurations.
- Helm values and Kubernetes manifests.
- Components inside modules.
- Expressions in rules, such as CEL.

For an example, refer to [tests/clustering](tests/clustering). The rules
there come from the
[prometheus-metrics-clustering recipe](../../recipes/prometheus-metrics-clustering).
