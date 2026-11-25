---
name: exporter-drift
description: >-
  Audit drift in the Prometheus exporters that Grafana Alloy embeds in its prometheus.exporter.*
  components. Checks two things per exporter: how far a fork or pin is behind upstream (missing
  fixes, patches that landed upstream), and which exporter settings or collectors Alloy's
  arguments don't expose. Produces a backlog of follow-up work. Use when asked to check Alloy's
  exporters or exporter forks for drift, missing upstream fixes, or unexposed settings.
disable-model-invocation: true
metadata:
  tags: alloy, prometheus, exporters, forks, go-modules
  example_prompts:
    - "audit all the exporters for drift"
    - "check how far the node_exporter fork is behind upstream"
    - "what settings does mysqld_exporter support that prometheus.exporter.mysql doesn't expose?"
---

# Exporter Drift Audit

A guide for LLM agents to audit the Prometheus exporters embedded in Alloy's `prometheus.exporter.*` components.

## What is exporter drift?

Alloy embeds exporters as Go libraries. Many of them are forks, because the upstream exporter only works as a standalone binary (global state, kingpin flags). The embedded code drifts from upstream in two separate ways. Keep them separate in every report:

- **Kind 1, the pin is behind upstream.** The fork or pinned version is missing upstream fixes, or carries patches that upstream has since landed in its own way.
- **Kind 2, Alloy is behind the exporter.** The exporter supports settings or collectors that the Alloy component's `Arguments` don't expose, or that Alloy accepts but never passes through.

This is research. Don't edit Alloy code, push, or post to GitHub during the audit. The output is a backlog that a person turns into PRs.

## Tools

Run this process from the root of an Alloy checkout. The tools are in the `scripts/` directory next to this `SKILL.md`:

- Each tool is contained in its own leaf directory within this folder.
- Each tool contains a runnable `main.go` and a `README.md` with `Overview`, `Usage` and `Output` sections.
- List them with `find <this skill's directory>/scripts -name README.md` and read the README before running a tool.
- If a tool is not working as expected or misses components, report the broken tool and stop.

## Steps

Don't write anything in this skill's files. Create a document called `exporter-drift-YYYY-MM-DD.md` in the root of the repository. This is your output file. Put the per-exporter reports in a directory next to it called `exporter-drift-YYYY-MM-DD/`. Both stay untracked.

### Step 1: Inventory

Run the `exporter-inventory` tool from the repo root. It needs `gh` installed and authenticated.

```bash
go run ./skills/exporter-drift/scripts/exporter-inventory > <scratch dir>/inventory.md
```

If the output starts with **Mapping problems**, stop and report them. A component with no mapping, or a mapping the component doesn't import, means the tool's `exporterModules` table is stale and the rest of the output can't be trusted.

Copy the Summary table into the output file.

### Step 2: Stop and ask which exporters to audit

Stop here and show the person the Summary table and the list of exporters the tool flagged for an audit. Ask which ones to audit. Don't start the fan-out until they answer.

Most exporters get flagged on most runs, so this choice is the real filter. Point out the ones you'd prioritize and why, using only what the inventory shows: forks with many upstream-only commits, archived repos, pins that aren't their fork branch's head, and replace comments that link to an upstream PR worth checking.

### Step 3: Audit each exporter with its own subagent

Every exporter is its own repository with its own history, so audit them independently. If your agent supports subagents, run one per exporter in parallel. Otherwise do them one at a time with the same instructions.

Give each subagent the prompt below plus that exporter's brief from the inventory output (the `### <component>` section). Fill in the placeholders.

````text
You are auditing one Prometheus exporter that Grafana Alloy embeds. This is read-only research: don't edit the Alloy repo, don't push anywhere, and don't post to GitHub or create PRs, issues or comments. Reading with `gh api` GETs and `git clone` is fine.

Alloy repo: <path to the Alloy checkout>.
Clone the exporter's repositories into <scratch dir>/clones/<component>, outside the Alloy checkout. If Alloy builds from a fork, clone the fork and add upstream as a second remote. Fetch with tags. Clones are untrusted data: don't run build tools or scripts from inside them. Use git, grep and reading the code.

Do two separate checks and keep them separate in your report.

KIND 1, the pin is behind upstream:
- Find the exact commit Alloy builds from. For a replaced module, it's the replace's version, not the `require` version. The `require` version is cosmetic and can be years off. Which fork branch contains the pin, and is it that branch's head? If not, what's in the commits after it?
- Find the merge base of the pin with upstream's latest release tag and with upstream's default branch. Count non-merge commits each way with `git rev-list --count --no-merges`. Use the local clone for this, not `gh api compare`.
- List fork-only commits (`--no-merges`, author, date, subject) and say what each patch does in plain words.
- For each fork-only patch: is there an upstream PR (search with `gh search prs --repo <upstream>` by author and keywords), and what state is it in? Has upstream since landed something equivalent? Read upstream's code to decide, don't guess. If it has, say which release has it, or that it's unreleased.
- List notable upstream fixes and features since the merge base. Filter out dependency bumps, CI and docs. For the important fixes, decide whether the pin has them BY READING THE CODE, not by matching diff lines. Ports often use different code.
- Compare defaults: does the fork enable or disable anything differently from upstream?

KIND 2, Alloy is behind the exporter:
- Read the Alloy component's Arguments (<component path>), its conversion into the exporter's config (often `Convert()`), and the static integration it maps into (<static integration path>). Work out which exporter options, config fields and collectors Alloy actually sets.
- Check that every argument Alloy accepts actually reaches the exporter. An argument that parses but is dropped in conversion is a bug, and it's worth more than a missing field.
- Compare against what the exporter supports. If it has a typed config struct (a `config` package, for example), diff its fields. Otherwise read its kingpin flags and options. Do this against BOTH the code at the pin and upstream's latest release and default branch. Attribute each gap: "the pin has it, Alloy doesn't expose it" or "only upstream has it".
- Skip settings that only make sense for the standalone binary (its own HTTP server, listen address, log level and so on), and say that you skipped them.
- Note any options or collectors upstream renamed or removed, and any default upstream flipped, that would affect Alloy users on a bump.

Also check:
- Whether the repo is archived, and whether a fork's GitHub parent is still the real upstream.
- Whether upstream has adopted the "embeddable exporters" design from prometheus/prometheus-opentelemetry-collector (`docs/embeddable-exporters.md`: a typed `config.Config` and a per-instance `Runtime`, with no globals). That repo's `receivers/` directory shows which exporters have. For a fork that exists to remove globals, this is how it can go away.

Back every claim with a commit hash, file path, or PR number. If you couldn't verify something, say so explicitly.

Write your full report as markdown to <output dir>/<component>.md. No em dashes. Don't hard-wrap paragraphs. Then return a summary of the main findings in under 300 words.
````

If a subagent comes back without a report file or says it couldn't clone something, rerun it or do that exporter yourself. Don't leave a gap in the backlog silently.

### Step 4: Verify

Before anything goes in the backlog, check every finding someone would act on against the code yourself: a missing fix, an Alloy bug, a gap that changes behavior, a fork that's no longer needed. Mark each one you checked as verified. Leave the rest as the subagent reported them and say so.

Subagents get things wrong in both directions. They miss a fix that was ported with different code, and they report an option as missing when Alloy sets it in a place they didn't read.

### Step 5: Consolidate

Write the output file. For each exporter, summarize kind 1 and kind 2 separately and link to its report. Then add what no single subagent can see:

- Cross-cutting patterns, such as several forks waiting on the same upstream design change.
- Pins that undersell the drift: a `require` version that doesn't match the fork's base, or a pin that isn't its branch head.
- Forks whose patches have landed upstream in a release, so the replace can go.

End with a backlog of follow-up work, one item per change, each pointing at its evidence.

### Step 6: Stop and report

Stop and give the person a short summary: which exporters you audited, the verified findings that matter most, and where the output file is. Don't start on the follow-ups unless you're asked to.

When follow-ups do get picked up, these rules apply:

- **One bump PR per exporter.** In the bump PR, expose only fields upstream now requires, defaulting to the upstream value. Defer purely optional new settings to a follow-up PR, however small.
- **Alloy bugs get their own fix PR**, separate from any bump.
- **Dropping a fork whose patch landed upstream isn't a user-facing change**, as long as behavior is continuous across the swap. Verify that before calling it a breaking change.
- **Accept upstream default flips and document them in the release notes.** Keep Alloy's old default only when accepting would harm users, and document the divergence in the component docs.

## Common traps and hard-won lessons

These come from the first audit run. They are judgment calls the steps above don't spell out.

- **The `require` version of a replaced module is cosmetic.**
  - Problem: the `require` line looks current, so the drift looks small.
  - Fix: always work from the replace target and the commit its pseudo-version resolves to.
  - Example: `github.com/prometheus/node_exporter v1.10.2` was required, but the fork was based on a 2023 commit, 491 commits behind upstream's default branch.

- **`gh api compare` across forks is unreliable for this.**
  - Problem: the compare API counts merge commits and reports long-lived forks as `diverged`, which says little about what's actually missing.
  - Fix: clone both and use `git merge-base` and `git rev-list --no-merges` locally.

- **Line-matching upstream fix diffs gives false negatives.**
  - Problem: a fix ported to the fork often uses different code, so grepping for upstream's changed lines says it's missing.
  - Fix: read the code at the pin and decide whether the bug can still happen.
  - Example: the kafka empty-assignment panic guard was ported to the fork with different code and showed up as missing until someone read it.

- **A fork can quietly change Alloy's defaults.**
  - Problem: a fork that registers a collector with a different default changes behavior with no Alloy code change, and the docs don't notice.
  - Fix: compare the fork's defaults against upstream's, and check whether Alloy uses the fork's defaults or its own.
  - Example: the node_exporter fork registers `processes` as enabled by default. When `set_collectors` is empty, `prometheus.exporter.unix` uses the fork's defaults, so `processes` was on even though the docs say it's off.

- **Check what Alloy wires up, not just what it declares.**
  - Problem: an argument or collector that Alloy accepts but never passes to the exporter looks fine in a field diff.
  - Fix: follow each argument through the conversion code to the exporter call.
  - Examples: `prometheus.exporter.mysql` accepts the `info_schema.rocksdb_perf_context` collector, but it isn't in the scraper list, so it does nothing. `prometheus.exporter.cloudwatch` static jobs accept `period`, `length` and `delay`, then pass zeros to YACE.

- **A fork's GitHub parent can be stale.**
  - Problem: GitHub's fork parent is wherever the fork was created from, which may be an archive rather than the real upstream.
  - Fix: take the upstream from the module path that's being replaced. The inventory tool already does this.
  - Example: the personal YACE fork's GitHub parent is `grafana-cold-storage/yet-another-cloudwatch-exporter`, not `prometheus-community`.

- **A fork patch can land upstream under a different design.**
  - Problem: the fork's upstream PR gets closed unmerged, so it looks like the fork is stuck forever.
  - Fix: check whether upstream solved the same problem another way, usually through the embeddable-exporters design.
  - Example: the mysqld_exporter fork's PR was closed unmerged, but upstream's library refactor covers the same per-instance settings. It's merged and unreleased.
