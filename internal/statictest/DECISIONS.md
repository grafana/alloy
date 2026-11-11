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
`flows-through`. They cover the clustering recipe and the user rules we know
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

## Severity

A failed `should` rule is a warning: it shows `!` and doesn't fail the test.
The exit status then only depends on `must` rules, so CI can gate on them.

## Recipes

A recipe stores its rules in `rules.yaml`, with an optional `static` check in
the static test format. The static test agent copies these checks into a test
file, so the copy must match the recipe exactly. `TestRecipeRules` checks
this for the examples.

We put rules in YAML, not in `RECIPE.md`. Then Go tests can check them, and
an agent copies them without parsing prose. The recipe format only has
what the static test agent needs. The rest of the recipe format is not
designed yet.

## Not done yet

- OTel Collector configurations, Helm values and Kubernetes manifests.
- Components inside modules.
- `alloy test static init`, which would collect the static checks of the
  selected recipes into a test file. The agent does this for now.
