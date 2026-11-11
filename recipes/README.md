# Recipes

**Experimental.** A recipe holds our knowledge about one configuration
task, such as clustering for Prometheus metrics. An agent reads it to build
and check a configuration. This first version defines only the parts that
the static test agent reads. The format can change in any release.

Recipes have the same version as Alloy. A tool that uses them reads them
from the git tag of the Alloy release that it targets.

## Layout

```
recipes/
  <recipe-name>/
    RECIPE.md     name and description, instructions, test guidance
    rules.yaml    what must be true in the output
```

## `RECIPE.md`

The frontmatter has a `name`, which equals the directory name, and a
`description`. An agent uses the description to select recipes, so write
what the recipe does and when it applies.

The body has these sections:

- **Instructions:** how to build the configuration, for the agent that
  writes it.
- **Pipeline test guidance:** what a pipeline test of this recipe can and
  can't check.

## `rules.yaml`

```yaml
rules:
  - name: clustering on all Prometheus scrape components
    description: Why the rule exists, so a user can decide on an exception.
    docs: https://grafana.com/docs/alloy/latest/get-started/clustering/
    severity: must          # must or should
    scope: alloy            # alloy or deployment
    static:                 # optional: a check for alloy test static
      type: all-or-none
      select: [prometheus.scrape]
      path: clustering.enabled
      equals: true
```

- `static` holds a check in the static test format, without its name,
  description, docs and severity. For the rule types, refer to
  [internal/statictest/README.md](../internal/statictest/README.md).
- A rule without `static` is for the verifier agent.
- Only `scope: alloy` rules can have `static`. Static tests don't check
  deployments yet.

`go test ./internal/statictest/` loads every `rules.yaml` and checks that
each static check is valid.
