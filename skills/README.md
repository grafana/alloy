# skills

Agent skills for working on Alloy. Each subdirectory is one skill:

```
skills/
  <skill-name>/
    SKILL.md        required; `name` in frontmatter must equal the directory name
    references/     optional; loaded on demand rather than with the skill
    scripts/        optional; anything the skill invokes
```

## Installing skills

These follow the [Agent Skills specification](https://agentskills.io) and install with
[`npx skills`](https://github.com/vercel-labs/skills), which supports Claude Code, Cursor, Pi,
Codex, Gemini CLI and many other agents.

From the root of your Alloy clone, install every skill into the project for the agents you use:

```bash
npx skills add . --skill '*' -a claude-code cursor pi codex gemini-cli
```

`skills/` is the source of truth. Make every change there, never in the installed copies.

The install copies each skill into `.agents/skills/`, where Cursor, Pi, Codex and Gemini CLI read
it. `.claude/skills/` holds symlinks to those copies for Claude Code. The install is recorded in
`skills-lock.json`. All three paths are in `.gitignore`.

Because the installed skills are copies, they don't pick up later changes to `skills/`. Run the
install command again after you pull or edit skills.

To install them globally instead, for use outside an Alloy clone:

```bash
npx skills add grafana/alloy --skill '*' -g -a claude-code cursor pi codex gemini-cli
```

Run `npx skills --help` for other options, such as listing, updating or removing skills.

These are also plain markdown, so any agent can follow one directly by being pointed at the
`SKILL.md`.

## Adding a skill

Skills in this repository must be relevant to the Alloy repository, its OSS contributors, or Alloy
users. Other skills need a different home.

Keep every skill under `skills/` in the format shown above so the CLI can discover it. Each
`SKILL.md` must start with YAML frontmatter containing `name` (matching the directory) and
`description`; optional `references/` and `scripts/` directories are installed alongside it.

`npx skills init <name>` scaffolds one. The description is the whole routing signal, so it must
state both what the skill does and when to use it. Keep it well under the spec's 1024-character
cap, since every installed skill's description sits in the agent's context permanently.

Skills here follow the same rules as any other contribution, including the
[Generative AI policy](../docs/developer/genai.md).
