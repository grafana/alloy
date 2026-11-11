---
name: alloy-static-test
description: >-
  Write a static test (*.statictest.yaml) for one Alloy configuration. It
  collects the static checks of the selected Alloy recipes and the user's own
  rules, runs `alloy test static`, and reports each rule that fails. Use it
  after an Alloy configuration is generated or changed, to check a
  configuration against recipe rules, or on an upgrade to a new Alloy
  version.
metadata:
  tags: alloy, static-test, recipes, configuration
  example_prompts:
    - "write a static test for config.alloy with the clustering recipe"
    - "check config.alloy against the recipes for Alloy v1.12.0"
    - "add a rule that every log path goes through our secret filter"
---

# Static test agent

Write a static test for one generated configuration, run it, and report the
result.

## Inputs

- The path of the Alloy configuration. OTel Collector configurations aren't
  supported yet.
- The names of the recipes that were selected for the configuration.
- The target Alloy version, such as `v1.12.0`. If it isn't given, use the
  output of `alloy --version`.
- Optional: rules from the user, such as "every log path goes through our
  secret filter".

Read the recipes and the static test format from the git tag of the target
version, not from `main`:

- Recipes: `https://github.com/grafana/alloy/tree/<tag>/recipes/<recipe>/rules.yaml`
- Format and rule types:
  `https://github.com/grafana/alloy/blob/<tag>/internal/statictest/README.md`

In an Alloy checkout at that tag, read `recipes/` and
`internal/statictest/README.md` directly.

## Steps

1. For each selected recipe, read its `rules.yaml`. Collect the rules that
   have a `static` check. Skip the rules without one, because the verifier
   agent checks them.
2. Write one file next to the configuration, named after it, such as
   `config.statictest.yaml` for `config.alloy`. Start it with:

   ```yaml
   kind: statictest
   version: 1
   config:
     path: config.alloy   # relative to the test file
   rules:
   ```

3. For each collected rule, add one entry to `rules`. Copy `name`,
   `description`, `docs` and `severity` from the recipe rule. Then copy every
   field of its `static` check, unchanged, into the same entry. Don't copy
   `scope`. Add a comment line that names the recipe above its rules.
4. Add the user's rules to the same file, under a `# User rules.` comment.
   Use only the rule types in the format reference of the target version. If
   a user rule needs a type that doesn't exist, don't invent one. Tell the
   user that the rule can't be a static check yet.
5. Run `alloy test static --output=json <file>`.
6. The result can be an error, such as an invalid rule or a config that
   doesn't parse. Then fix the test file and run it again. Don't change the
   configuration.
7. Report the result.

## Report

For each rule that fails, give:

- its name and severity. A `should` rule is a warning.
- the message, which names each component and line that broke the rule.
- its description and docs link, so that the user can decide if their
  configuration is a valid exception.

Then list the rules that pass, by name only. Also list the user rules that
you couldn't write as a static check.

## Constraints

- The user owns the test file. Don't remove or weaken a rule that fails,
  and don't change its description. Only the user accepts an exception, when
  they change or remove the rule.
- Don't change the configuration to make a rule pass. Report the failure.
- Use only the rule types that the target Alloy version supports. An older
  `alloy` binary rejects an unknown type with an error.
- Don't check Helm values or Kubernetes manifests. Static rules don't
  support them yet. Rules with `scope: deployment` are for the verifier
  agent.

## On upgrade

When the user moves to a new Alloy version:

1. Collect the static checks again from the recipes of the new version. The
   old test file doesn't get new rules by itself.
2. Compare them with the rules in the existing file, by name:
   - Add the recipe rules that are new.
   - For a recipe rule that changed, show the user the old and the new
     check, and ask before you replace it. The user may have changed it on
     purpose.
   - Keep the user's rules and the rules that the user changed.
3. Run the test with the new `alloy` binary, and report as above. An old
   binary may not support new rule types.
