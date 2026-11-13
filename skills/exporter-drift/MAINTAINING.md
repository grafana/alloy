# Maintaining the exporter-drift skill

This file is for people editing the skill. The agent follows [`SKILL.md`](SKILL.md) and the tools in [`scripts/`](scripts/).

## How to use this

Install the skill as described in [`skills/README.md`](../README.md), or point your agent at `SKILL.md` directly. The skill only runs when you invoke it by name: `/exporter-drift` in Claude Code and Cursor, `/skill:exporter-drift` in Pi, or `$exporter-drift` in Codex. From the root of an Alloy checkout, give it a prompt like this:

```text
/exporter-drift audit all the exporters.
```

It stops after the inventory and asks which exporters to audit. Each one gets its own subagent, so auditing all of them is a lot of tokens. Picking a handful is usually the better run.

This is separate from `key-deps-update` on purpose. Exporter forks are out of scope there, and this audit is meant to be run on its own schedule.

## Prompt guidelines

The prompt guidelines in [`key-deps-update/MAINTAINING.md`](../key-deps-update/MAINTAINING.md) apply here too: stop-and-ask points between milestones, instructions next to the step that uses them even when that repeats things, and Go tools for anything mechanical.

The subagent prompt in Step 3 is self-contained on purpose, because the subagent doesn't see the rest of `SKILL.md`. Don't replace parts of it with references to other sections.

When an audit run turns up a new failure mode, add it to "Common traps and hard-won lessons" with a problem, a fix and an example.

## Future improvement ideas

- The inventory's audit gate flags most exporters on most runs, so in practice the person picks at the Step 2 stop. A tighter gate (for example, only forks and exporters behind a release) would make the default run smaller.
- cadvisor replaces both `github.com/google/cadvisor` and `github.com/google/cadvisor/lib`, and the inventory only reports the first.
- For exporters that adopt the embeddable-exporters design, the kind-2 check is close to a struct-field diff of upstream's `config.Config` against Alloy's `Arguments`. That could become a tool.
