# skills

Agent skills for working on Alloy. Each subdirectory is one skill:

```
skills/
  <skill-name>/
    SKILL.md        required; `name` in frontmatter must equal the directory name
    references/     optional; loaded on demand rather than with the skill
    scripts/        optional; anything the skill invokes
```

| Skill | What it does |
|---|---|
| [`key-deps-update`](key-deps-update/SKILL.md) | Updates Alloy's key Go dependencies (OTel Collector, Prometheus, Beyla, Loki): versions, forks, `go.mod`, build and test fixes, and user-facing change notes |

## Installing a skill

These follow the [Agent Skills specification](https://agentskills.io) and install with
[`npx skills`](https://github.com/vercel-labs/skills), which supports Claude Code, Codex, Cursor,
OpenCode and many other agents. `skills/<name>/` is one of the CLI's standard discovery paths, so
the repository shorthand works directly:

```bash
# List what's here
npx skills add grafana/alloy --list

# Install one skill globally for Claude Code
npx skills add grafana/alloy --skill key-deps-update -g -a claude-code

# Or run it once without installing
npx skills use grafana/alloy@key-deps-update | claude
```

From a local clone, `npx skills add . --list` works the same way. Installing symlinks to a
canonical copy by default, so `npx skills update` picks up changes; pass `--copy` where symlinks
aren't supported.

Once installed, the agent can select the skill when a request matches its description. Skills
that change code, like `key-deps-update`, still expect to run from the root of an Alloy checkout.

These are also plain markdown, so any agent can follow one directly by being pointed at the
`SKILL.md`.

## Adding a skill

Keep every skill under `skills/` in the format shown above so the CLI can discover it. Each
`SKILL.md` must start with YAML frontmatter containing `name` (matching the directory) and
`description`; optional `references/` and `scripts/` directories are installed alongside it.

`npx skills init <name>` scaffolds one. The description is the whole routing signal, so it must
state both what the skill does and when to use it. Keep it well under the spec's 1024-character
cap, since every installed skill's description sits in the agent's context permanently.

Notes for people maintaining a skill, as opposed to the agent running it, go in a separate file in
the skill directory, such as [`key-deps-update/MAINTAINING.md`](key-deps-update/MAINTAINING.md).

Verify discovery before pushing:

```bash
npx skills add . --list
```

Skills here follow the same rules as any other contribution, including the
[Generative AI policy](../docs/developer/genai.md).
