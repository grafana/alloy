# Pipeline tests

A pipeline test runs an Alloy configuration in process, sends it data, and
checks what comes out. It needs no network, containers or backend, so it
usually finishes in under a second.

You can run pipeline tests in two ways:

- `go test ./internal/pipelinetest/` runs every test under `tests/`. Alloy
  uses this to test itself.
- `alloy test` runs any test file. It's **experimental**: the command, the
  file format and the JSON output can change in any release. For the design
  and how to change it, refer to [DECISIONS.md](DECISIONS.md).

## Run tests

```sh
alloy test                       # every test file under the current directory
alloy test ./checkout.pipelinetest.yaml ./logs/
alloy test --match='*.yaml' ci/  # select files in a directory by pattern
alloy test --output=json .       # one JSON report, for CI or an agent
alloy test --timeout=1m -v .     # retry assertions longer, and show Alloy's logs
```

`alloy test` runs each file you name. It searches each directory, and its
subdirectories, for files that match `--match`. The default matches names
ending in `.pipelinetest.yaml` or `.pipelinetest.yml`.

- A pattern without `/` matches the file name. A pattern with `/` matches the
  path from the directory, and supports `**`.
- A matched file without `kind: pipelinetest` shows SKIP, and doesn't fail
  the run.
- Directories whose names start with `.` aren't searched.
- You can't use `--match` with a file path.

The command exits with 0 when every test passes, and 1 otherwise.

Each assertion shows ✓ or ✗. A failed test also shows what reached the sink,
which components are unhealthy, and Alloy's first warnings and errors. A test
that can't run shows ERROR, with the line in your config that caused it.

## Test a real configuration

You can test the config you deploy without changing it. The test file swaps
the config's edges for the harness:

1. **Choose the config.** Set `config.path`, relative to the test file.
2. **Swap the exits.** List each `loki.write`, `prometheus.remote_write` and
   `otelcol.exporter.*` under `config.sinks`. Each exit gets its own sink,
   which captures the data and adds the exit's `external_labels`.
3. **Swap the entries.** Add an input with `replace: <component>` for each
   source, such as `loki.source.file` or `prometheus.scrape`. The input sends
   its data where the source sent it.
4. **Handle what needs a real environment.**
   - Replace a `discovery.*` component with a `targets` input to test the
     relabel rules after it.
   - Replace a file or endpoint that only exists in production with
     `config.override`, such as
     `loki.secretfilter.default: {gitleaks_config: ./gitleaks.toml}`.
   - Set the variables the config reads with `sys.env` under `env`.
   - List anything else under `config.remove`, along with whatever
     references it.
5. **Write inputs.** For each pipeline, send a record it keeps, one it drops,
   and one it rewrites.
6. **Assert** what comes out. Add `sink: <exit>` to check routing, such as
   audit logs reaching only the audit writer.
7. **Run `alloy test`** until it passes.

```yaml
kind: pipelinetest
version: 1

env:
  CLUSTER_NAME: prod-eu

config:
  path: config.alloy
  sinks: [loki.write.cloud]
  remove: [discovery.kubernetes.pods, discovery.relabel.pods]

inputs:
  loki:
    - replace: loki.source.file.app
      entries:
        - {labels: {service_name: app}, line: 'level=info msg="served"'}
        - {labels: {service_name: app}, line: 'level=debug msg="cache miss"'}

assert:
  loki:
    - name: the level becomes a label
      sink: loki.write.cloud
      match: {selector: '{service_name="app", level="info", cluster="prod-eu"}'}
    - name: debug lines are dropped
      count: 0
      match: {selector: '{level="debug"}'}
```

For metrics and traces as well, refer to [tests/real-config](tests/real-config).

### Components inside modules

To swap a component inside a `declare` block, write
`<module>/<component>`, such as `cluster_metrics/discovery.kubernetes.nodes`.
This works in `replace`, `config.remove` and `config.override`. For a
`declare` inside another `declare`, write `outer/inner/<component>`.

```yaml
inputs:
  targets:
    - replace: cluster_metrics/discovery.kubernetes.nodes
      targets: [{__address__: 10.0.0.1:10250, __meta_kubernetes_node_name: node-a}]
  prometheus:
    - replace: cluster_metrics/prometheus.scrape.nodes
      samples: [{series: 'kubelet_running_pods{job="kubelet"}', value: 12}]

assert:
  targets:
    - from: cluster_metrics.feature/discovery.relabel.nodes.output
      match: {labels: {node: node-a}}
```

- A replace or remove changes the module for every block that uses it. An
  input sends its data once for each of these blocks.
- A `targets` assertion reads a component in a module through the block that
  uses the module, such as `cluster_metrics.feature/...`.
- Exits must be top-level. Pass them to the module as an argument, as the
  Kubernetes Monitoring Helm chart does.
- Modules from `import.*` blocks aren't supported.

For an example, refer to [tests/declare-module](tests/declare-module).

### Keep write relabel rules

Set `config.keep_write_relabel: true` to make the sink of each
`prometheus.remote_write` apply its `write_relabel_config` blocks. The sink
applies them after `external_labels`, as `prometheus.remote_write` does. A
sample the rules drop doesn't reach the sink. Use it to test rules such as a
`scraped_by` label. The exit must have one `endpoint` block.

### What a test doesn't cover

- **The swapped components:** an exit's sink applies its `external_labels`,
  and its `write_relabel_config` with `keep_write_relabel`. The test doesn't
  cover batching, auth or the wire protocol. A replaced `prometheus.scrape`
  doesn't add `job` and `instance`, so set them on your samples if the
  pipeline uses them.

Paths in the test file and in the config, such as `import.file` filenames,
are relative to the test file's directory.

### Write a test with an agent

An agent can do steps 2 to 7. This prompt works:

> Write an `alloy test` file for `config.alloy`, using the format in
> `internal/pipelinetest/README.md` of the Alloy repository. Send a few
> inputs per pipeline that each exercise its processing, and assert on the
> result. Run `alloy test --output=json` until it passes. If a failure looks
> like a bug in the config, say so instead of changing the assertion.

## File format

A test file has these sections. `kind`, `version`, `config` and `assert`
are required. Unknown fields fail the test.

```yaml
kind: pipelinetest
version: 1       # the file format version
flags: {}        # Alloy run flags, such as feature.loki.pipeline-v2.enabled: true
env: {}          # environment variables the config reads with sys.env
config: ...      # the Alloy configuration under test
mocks: {}        # stub upstreams the config can call
inputs: {}       # data to send into the pipeline
assert: {}       # what must come out
```

### `config`

Use inline Alloy source, or a file with components to swap as shown above.
Inline configs send to the sink through these references:

| Reference                          | Use as                            |
| ---------------------------------- | --------------------------------- |
| `pipelinetest.loki.receiver`       | a `forward_to` entry for logs     |
| `pipelinetest.loki.url`            | a `loki.write` endpoint URL       |
| `pipelinetest.prometheus.receiver` | a `forward_to` entry for metrics  |
| `pipelinetest.prometheus.url`      | a `prometheus.remote_write` URL   |
| `pipelinetest.otel.input`          | an `output` entry for OTLP traces |

```yaml
config: |
  loki.process "default" {
    forward_to = [pipelinetest.loki.receiver]
  }
```

`config.override` only replaces attributes the config already sets.

### `inputs`

Each input sends to the receivers in `components`, or replaces a component
with `replace`. Set one of them. A `targets` input only takes `replace`.

```yaml
inputs:
  loki:
    - components: [loki.process.default.receiver]
      entries:
        - labels: {service_name: api}
          line: 'level=error msg="boom"'
          line_repeat: 1000                     # optional, repeats line, for a long line
          timestamp: 2026-04-22T12:00:00Z       # optional, defaults to now
          structured_metadata: {trace_id: abc}  # optional
  prometheus:
    - components: [prometheus.relabel.default.receiver]
      samples:
        - series: 'http_requests_total{method="post"}'   # or labels: {__name__: ..., ...}
          value: 1027
          type: counter                                  # optional: type, help, unit
  traces:
    - components: [otelcol.processor.batch.default.input]
      spans:
        - name: GET /checkout
          service_name: shop              # the service.name resource attribute
          kind: server                    # internal (default), server, client, producer, consumer
          status: error                   # unset (default), ok, error
          attributes: {http.route: /checkout}
          resource_attributes: {k8s.namespace.name: shop}
  targets:
    - replace: discovery.kubernetes.pods  # references to its targets get this list
      targets:
        - __address__: 10.0.0.1:8080
          __meta_kubernetes_namespace: shop
          __meta_kubernetes_pod_container_port_name: http-metrics
```

To send data to a component's own listener, use `http`, `tcp` or `udp`
inputs. For examples, refer to the `loki-source-*` tests.

### `mocks`

`mocks.http` starts stub HTTP servers for components that call an API or
probe an endpoint. Reference one in the config as
`pipelinetest.mocks.http.<name>.url`. For an example, refer to the
`prometheus-exporter-blackbox` test.

### `assert`

Group assertions by signal: `loki`, `prometheus`, `traces` and `targets`.

- An assertion with `count` passes when exactly that many items match. With
  no `match`, it counts everything of that signal.
- Any other assertion passes when at least one item matches.
- `name` labels the assertion in reports.
- `sink` limits the assertion to one exit's sink.

The harness retries all assertions until they pass together, or the timeout
expires. A `count: 0` can pass before the unwanted data arrives. Pair it
with an assertion on data you send at the same time.

| Signal       | Match fields                                                                            |
| ------------ | --------------------------------------------------------------------------------------- |
| `loki`       | `selector`, `labels`, `line` (exact), `line_regex`, `timestamp`, `structured_metadata`  |
| `prometheus` | `selector`, `labels`, `value`, `timestamp`, `histogram`, `metadata` (type, unit, help) |
| `traces`     | `name`, `service_name`, `kind`, `status`, `scope`, `attributes`, `resource_attributes`  |
| `targets`    | `selector`, `labels`, plus `from`: the export to read, such as `discovery.relabel.pods.output` |

```yaml
assert:
  prometheus:
    - match:
        selector: 'probe_success{job="blackbox"}'
        value: 1
  targets:
    - from: discovery.relabel.pods.output
      match:
        labels: {job: shop/api, __address__: 10.0.0.1:8080}
```

`selector` is a PromQL series selector with `=`, `!=`, `=~` and `!~`. It's the
shortest way to match, and it ignores labels it doesn't name.

Label and attribute maps take a plain map, or a mode and values:

```yaml
labels: {level: error, service_name: api}
labels:
  mode: partial       # exact or partial
  values: {level: error}
```

Log and metric labels default to `exact`. Span attributes and target labels
default to `partial`, because they carry many labels a test doesn't care
about.

### Not supported yet

- **Profiles:** you can't swap `pyroscope.write`, and there are no profile
  inputs or assertions.
- **OTLP metrics and logs:** they reach the sink and the report counts them,
  but you can't assert on them. Convert them to Prometheus or Loki in the
  config to test them.
- **Modules from `import.*`:** you can only swap components inside `declare`
  blocks of the config.
- **Exits inside modules:** you can't swap them for a sink.
