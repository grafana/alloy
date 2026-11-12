# Pipeline test decisions

This page records the design choices behind the experimental
`alloy test pipeline`: what we chose, why, and how to change it. None of them is hard to reverse
yet, because the command is experimental and nothing outside this package
depends on the format.

For how to write and run tests, refer to [README.md](README.md).

## Scope

The first version includes:

- **Command:** `alloy test pipeline` with text and JSON output.
- **Real configs:** a test swaps a config's entry and exit components. It can
  also replace discovery with fake targets, override attributes and remove
  components. The config file doesn't change.
- **Signals:** logs, metrics and OTLP traces, plus discovery targets.
- **Routing:** each exit gets its own sink, so assertions can check where data
  goes.
- **Shorter assertions:** `selector`, an inferred type, plain maps, `name` and
  `line_regex`.
- **Example:** [tests/real-config](tests/real-config) tests a realistic config
  end to end.

Likely next steps:

1. **Profiles:** a `pyroscope.write` sink, profile inputs and assertions.
2. **OTLP metrics and logs:** the sink counts them but can't assert on them.
3. **`alloy test init config.alloy`:** generate the sinks, replacements and
   removals from component names. A real metrics config needed 154 removals.
4. **Which rule fired:** assert on Alloy's own metrics, such as
   `loki_process_dropped_lines_total{reason=...}`.
5. **Coverage:** report the match blocks and relabel rules no input reached.
6. **Modules from `import.*`:** swap components inside imported modules.
7. **`hold_for`:** keep a `count` passing for a while. Refer to
   [Retries](#retries).

## Command

We chose `alloy test pipeline [path ...]`, labelled experimental in its
help and not gated behind `--stability.level`. `alloy test` with no
subcommand runs pipeline and static tests. For static tests, refer to
[../statictest/DECISIONS.md](../statictest/DECISIONS.md).

Users run configs with `alloy run` and check them with `alloy validate`, so
they expect `alloy test` next to those. A stability flag adds friction
without protecting anything, because the command doesn't change a running
Alloy.

To rename or move the command, change `internal/alloycli/cmd_test_command.go`.
It wraps `pipelinetest.RunFiles`.

### Finding test files

`alloy test` always runs a file you name. It searches a directory
recursively for files that match `--match`, which defaults to
`*.pipelinetest.yaml` and `*.pipelinetest.yml`.

- A pattern without `/` matches the file name, and a pattern with `/` matches
  the path from the directory, with `**`. The patterns use `doublestar`,
  which `local.file_match` also uses.
- A matched file without `kind: pipelinetest` shows SKIP, with a warning. It
  doesn't fail the run, so `--match='*.yaml'` works in a directory that also
  holds other YAML.
- The search skips directories whose names start with `.`, such as `.git` and
  `.github`.
- `--match` with a file path is an error, because a named file always runs.

The first version matched `test.yaml` and `*.test.yaml`. That needed a
directory per test, and it matched the Docker integration tests, which use
`test.yaml` with a different format.

To change the default, edit `DefaultMatch` in `report.go`.

### Format version

Every test file starts with `kind: pipelinetest` and `version: 1`. The
harness reads these two fields before the strict parse. So an older Alloy
reports `version 2 not supported`, and not an unknown field. A breaking change
to the format increments the version. The JSON report has its own `version`
field, for the same reason.

## Harness in the binary

The `pipelinetest.*` components ship in the `alloy` binary.
`harness.Register` registers them on first use, not in `init`, so only a
process running tests has them. `alloy run` can't use them.

A separate binary is one more thing to install and keep at Alloy's version,
and tests must run the components users run. If binary size becomes a
problem, build `alloy test` as its own binary from the same package.

## Swapping components

The test lists what to swap, so the config under test is the one you deploy:

- `config.sinks` replaces exits with sinks.
- `inputs.<signal>[].replace` replaces an entry component with that input.
- `inputs.targets[].replace` replaces a discovery component with fixed
  targets.
- `config.override` sets a component's top-level attribute.
- `config.remove` deletes a component.

The harness edits the config's text at positions from the parsed syntax
tree. A removed block becomes as many blank lines as it spanned, and
generated components go after the config. This keeps comments, formatting
and line numbers, so a load error shows the right line of your file.

We considered one `config.replace` map for everything. It's one place to
look, but inputs would then need a second way to say what they replace.
Printing a modified syntax tree is simpler code, but loses comments and line
numbers.

The logic lives in `config_transform.go`.

### Modules

A component inside a `declare` block is `<module>/<component>`, such as
`cluster_metrics/discovery.kubernetes.nodes`. Static tests use the same
IDs. The Kubernetes Monitoring Helm chart puts each feature pipeline in a
`declare` block, so without this its pipelines can't be tested.

- **Scopes:** each `declare` body is its own scope. A reference is only
  rewritten in the scope of the component it names. So
  `discovery.kubernetes.nodes` at the top level and in a module are
  different components.
- **Inputs:** a module can't reference a component outside it. So the
  `pipelinetest.source` of an input goes into the module, on the first line
  of the block it replaces. The lines after it don't move. Each block that
  uses the module runs its own copy of the source, and the input sends to
  each one.
- **Assertions:** `targets` assertions use the runtime ID, such as
  `cluster_metrics.feature/discovery.relabel.nodes.output`. That's the ID
  Alloy's UI and logs show.
- **Exits:** the sink is a top-level component, which a module can't
  reference. The chart passes its destinations in as arguments, so we don't
  support exits inside modules yet.
- **`import.*`:** an imported module's source isn't in the config file. We
  could support it later by reading `import.file` sources.

### Exits

| Component                 | Reference rewritten | Sink export                                  |
| ------------------------- | ------------------- | -------------------------------------------- |
| `loki.write`              | `<id>.receiver`     | `pipelinetest.sink.<id>.loki_receiver`       |
| `prometheus.remote_write` | `<id>.receiver`     | `pipelinetest.sink.<id>.prometheus_receiver` |
| `otelcol.exporter.*`      | `<id>.input`        | `pipelinetest.sink.<id>.otel_input`          |

`<id>` is the exit's ID with underscores for dots, such as
`loki_write_default`. The harness rejects other components, and references
to other exports of an exit.

Each exit gets its own sink, so `sink: <exit>` on an assertion checks
routing. Without `sink`, an assertion checks every sink.

**The sink applies the exit's `external_labels`, and nothing else.** The
harness copies the expression as written, and Alloy evaluates it. So
`sys.env` works with `env` from the test file. A label the data already has
wins, as in `loki.write` and `prometheus.remote_write`.

With `config.keep_write_relabel`, the sink also applies the
`write_relabel_config` blocks of a `prometheus.remote_write`. The harness
copies the blocks into the sink as written, and the sink runs them with
Prometheus's relabel code, after the external labels. That's the order
`prometheus.remote_write` uses. It's off by default, so a test of the
pipeline doesn't depend on the destination's rules. A remote write with more
than one `endpoint` is rejected, because each endpoint has its own rules and
gets its own copy of the data.

We don't plan to emulate more until users ask for it:

- **The real writer, with its URL set to the sink:** it tests the most. But
  the harness must remove auth, `batch_wait` makes every test slower, and
  OTLP exporters need an OTLP server in the sink.

### Entries

An input takes the replaced component's `forward_to`, or `output.traces` for
OTel receivers, as written, with sink references rewritten. What the entry
component does itself isn't tested: a replaced `prometheus.scrape` doesn't
add `job` and `instance`.

We could keep the scrape and point it at a generated
`prometheus.exporter.static`. That tests the scrape's own labels, but forces
a short scrape interval on it, and the test waits for a scrape.

### Discovery

A `targets` input replaces references to a discovery component's `targets`
export with a literal list. The relabel rules after it run unchanged.
`targets` assertions read any component's exported targets, such as
`discovery.relabel.pods.output`.

Testing a real metrics config showed most of its logic lives in
`discovery.relabel`, which removing discovery skipped. A scrape fed these
targets tries to reach the fake addresses, so replace or remove it.

### Overrides

`config.override` sets a top-level attribute to a literal. Use it for files
and endpoints that only exist in production, such as a secret filter's
`gitleaks_config`. It only replaces attributes the config sets, which keeps
the edit in place. It doesn't support nested blocks or expressions yet.

## Signals

The sink flattens each span into one record, with attributes as strings.
Assertions then don't walk the resource and scope hierarchy.

Span attributes and target labels match partially by default, and log and
metric labels exactly. SDKs, processors and discovery add many labels, so an
exact default would make those assertions long.

Metric inputs append samples straight to the destination, in one transaction
each. A sample's series uses the text exposition form,
`name{label="value"}`, which users and agents already know.

## Assertions

- **`selector`** is a PromQL series selector, and works for logs too, because
  a LogQL stream selector has the same syntax. It replaces 6 lines with 1:
  `prometheus-exporter-blackbox` went from 269 lines to 131.
- **The harness infers the type:** `count` when `count` is set, otherwise
  `contains`.
- **Plain maps** work for labels: `labels: {job: api}`. A map whose only keys
  are `mode` and `values` is read as the full form.
- **`line_regex`** matches lines with parts that vary, such as hashes.
- **`line_repeat`** on a log input repeats its line, so a truncation test
  doesn't need a large file.
- **`name`** labels an assertion in reports. Without it, the report describes
  the match.

The old forms still work.

### Retries

The harness retries all assertions together until they pass at once, or the
timeout expires (`--timeout`, 30s by default). So a `count: 0` can pass
before the unwanted data arrives. `hold_for` would fix that.

## Strict parsing

Unknown fields in a test file fail the test. Otherwise a misspelled `inputs`
key only appears as "0 entries" after the timeout. Every existing test
passed under this rule without changes.

## Changed defaults

- Log entries without a `timestamp` get the time they're sent, not the zero
  time. Real sources never send the zero time, and some stages drop old
  entries.
- `config.path` is relative to the test file.
- Paths inside the config are also relative to the test file. The harness
  changes the working directory to the test file's directory while the test
  runs. Before, paths were relative to the directory you ran `alloy test`
  in, so a test passed under `go test` and failed from the repository root.
- The harness gives Alloy the config file's path, so `module_path` is the
  config's directory. For an inline config, it's the test file's directory.

## Output

- **Text** streams one block per test, with ✓ or ✗ per assertion. A failed
  test also shows the unhealthy components, Alloy's first 10 distinct
  warnings and errors, and up to 20 captured items per signal, labelled with
  their sink. It uses color in a terminal unless
  `NO_COLOR` is set.
- **JSON** is one `pipelinetest.Report` at the end. A failed test includes
  the same warnings and errors, and up to 100 captured items per signal. A
  passing test doesn't, which keeps the report small.
- The exit status is 1 for failures and errors alike.

JSON Lines or JUnit XML would go next to `WriteJSON` and `TextReporter` in
`report.go`.

## Execution

Tests run one at a time, because the harness uses the default Prometheus
registry, the only one `prometheus.exporter.self` serves. This also makes it
safe to set `env` and the working directory on the process. Each test gets
a fresh data directory.
