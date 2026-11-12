# Static test decisions

This page records the design choices behind the experimental
`alloy test static`: what we chose, why, and how to change it. For how to
write tests, refer to [README.md](README.md).

## Command

`alloy test` has two subcommands, `pipeline` and `static`. With no
subcommand, it runs both kinds. Users then have one command for every check
of a config, and each kind can still run on its own.

The two kinds share file discovery, the report and the text output, in
`internal/pipelinetest/report.go`. `runFile` reads the file's `kind` and
calls `statictest.RunFile` for static tests. So `pipelinetest` imports
`statictest`, and not the other way. If more kinds come, move the shared code
into its own package.

## Our own rule types, not CEL

We start with four rule types: `attribute`, `all-or-none`, `count` and
`flows-through`. They cover the clustering rules and the user rules we know
of, such as a secret filter on every log path.

- A named type gives a clear error message, such as which component has no
  clustering block. A failed CEL expression only says false.
- An agent writes a named type more reliably than an expression.
- We can add a `cel` type later, next to the others.

To add a type, add it to `Check.Validate` in `schema.go` and to `checkRule`
in `rules.go`.

## Constants only

A rule only checks constant values. An attribute that calls a function or
reads an export fails the rule, with a message that says so. `sys.env` would
read the environment of the machine that runs the test, not the one that
runs Alloy. A rule that passes on that value would be misleading.

## Data flow

`flows-through` builds the graph from references. A reference to a
`receiver` or `input` export sends data to the referenced component, as in
`forward_to` and OTel `output` blocks. A reference to any other export, such
as `targets`, sends data from the referenced component. This rule is in
`pushExports` in `graph.go`. It's correct for the components we know, but a
component with another push export needs an entry there.

## Modules

A component inside a `declare` block is checked, with the ID
`<module>/<component>`, such as `cluster_metrics/prometheus.scrape.nodes`.
Pipeline tests use the same IDs to replace components. The Kubernetes
Monitoring Helm chart puts each feature in a `declare` block, so without this
a rule such as clustering on every scrape misses most of the config.

- **Scopes:** a reference inside a module only names components of the same
  module. A module's `argument` and `export` blocks aren't components.
- **Data flow:** each argument and export of a module is a node of the
  graph. A module component that sends to `argument.X.value` sends to the
  argument, and the argument sends to what each block using the module
  passes as `X`. A reference to `cluster_metrics.feature.receiver` goes to
  the module's export, and the export to what its value references. An
  argument or export pushes data when what it holds does, such as a
  `receiver`.
- **One copy:** a module's components are checked once, not once per block
  that uses the module. When two blocks pass different destinations, the
  graph joins them. So `flows-through` can report a path that only one
  combination has. It doesn't miss a path.
- **`import.*`:** an imported module's source isn't in the config file. We
  could read `import.file` sources later.

## Severity

A failed `should` rule is a warning: it shows `!` and doesn't fail the test.
The exit status then only depends on `must` rules, so CI can gate on them.

## Not done yet

- OTel Collector configurations, Helm values and Kubernetes manifests.
- Modules from `import.*` blocks.
- Shared rules that a test file can include, such as Grafana's
  recommendations for a task. For now, each test file holds its own rules.
