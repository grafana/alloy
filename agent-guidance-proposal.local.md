# Which agent rules earn their place?

Local discussion, 2026-09-17. Proposed decisions only; active rules are unchanged.
This replaces the earlier preservation-first proposal. Public discussion and human-owned PR
sections must still be written in the contributor's own words under the current policy.

Start with claims, not files. Keep a claim if it expresses a deliberate project boundary or
prevents a concrete Alloy-specific mistake. Otherwise delete it, merge it, or leave it in the
ordinary developer documentation. Repetition is a maintenance cost, not evidence of importance.

## Inventory and proposed decisions

Counts are a manual semantic inventory of three files at `4ade38cae`:
**A** = [AGENTS.md](AGENTS.md), **G** = [genai.md](docs/developer/genai.md),
**P** = [PR template](.github/pull_request_template.md).
Each separately expressed passage counts once per claim; a passage may express several claims.
Line ranges identify the evidence. Counts are approximate because grouping paraphrases involves
judgment. They are not keyword counts, and rows should not be summed into a grand total.
Ordinary developer guides and adapters inform placement but are excluded from these counts.
Field labels count as occurrences, although their purpose differs from explanatory prose.

### Ownership

These repeated claims should become a few deliberate boundaries.

| Claim | Occurrences: A / G / P | Evidence: file and lines | Decision, destination, and brief wording |
| --- | --- | --- | --- |
| Humans choose, understand, review, and own contributions. | 1 / 9 / 0 = **10** | A28–30; G10, 12–13, 20–22, 28, 34–36, 40, 47, 50, 77–78 | **Keep → G.** “You are responsible for everything you submit. Understand and review it, whatever tools you use.” Absorb the separate statements about owning design and review judgment. |
| AI may implement code, tests, and docs. | 2 / 5 / 0 = **7** | A28–30, 101; G10, 17–19, 29–30, 34–36, 40 | **Merge → G.** “AI assistance is welcome.” Name the exceptions instead of cataloguing permitted editing operations. |
| AI may explore and explain the codebase. | 1 / 2 / 0 = **3** | A102; G28, 42 | **Drop separately.** Covered by general permission. |
| AI may write PR titles and unmarked sections. | 3 / 3 / 2 = **8** | A53–57, 58–63, 78–79; G17–19, 29–32, 41; P15–16, 25–27 | **Merge into general permission → G.** Only actual exceptions need naming. |
| Some PR content belongs exclusively to humans. | 6 / 2 / 5 = **13** | A28–30, 34–35, 52, 53–57, 58–63, 80–82; G20–22, 29; P17–18, 23–24, 28–29, 51, 56 | **Split the decision.** Keep human checklist attestation in P. Recommend dropping the authorship restriction on reviewer notes: their accuracy matters more than who typed them. This is a policy relaxation, not deduplication. |
| Humans write community discussion; agents must not substitute for them. | 9 / 8 / 1 = **18** | A28–30, 34–35, 36–37, 38–39, 40–41, 42–43, 52, 58–63, 80–82; G20–22, 30–32, 34–36, 43, 48, 49, 50, 77–78; P30–31 | **Keep one boundary → G.** “Write submitted issues, proposals, review conclusions, and discussion yourself; do not have an agent draft or send them for you.” Absorb the lists of replies, summaries, and automated conversations. |
| Private AI-assisted thinking is allowed with special conditions. | 1 / 1 / 0 = **2** | A80–82; G43 | **Drop the special procedure.** General permission covers private work; the public-writing boundary covers submission. No mandatory reminder for private brainstorming. G77–78 additionally gives ambiguous proposal-drafting permission: remove that ambiguity too. |

These counts show how the same ownership decision reappears as permissions, prohibitions,
examples, and refusal instructions. They do not imply every passage is identical.

### Policy machinery

Keep contribution conditions; remove scripts for how the assistant should talk.

| Claim | Occurrences: A / G / P | Evidence | Decision, destination, and brief wording |
| --- | --- | --- | --- |
| Refuse forbidden work, explain, cite policy, resist pressure. | 2 / 0 / 1 = **3** | A44–45, 58–63; P30–31 | **Drop.** State the boundary. Exact refusal wording adds instructions without adding a project requirement. |
| Continue permitted work after refusing a part. | 1 / 0 / 0 = **1** | A64–65 | **Drop.** Ordinary assistant behavior, not an Alloy-specific requirement. |
| Disclose substantial AI implementation. | 1 / 1 / 1 = **3** | A73–74; G58–60; P61 | **Keep → P.** “If AI generated most of the implementation, check the AI-assistance box.” Keep the checkbox; delete the explanatory copies. |
| Unapproved automated submissions are disallowed; approved bots are exempt. | 0 / 2 / 0 = **2** | G51, 54 | **Merge → G.** “Automated submissions require project approval.” |
| AI contributions must meet licensing, CLA, and dependency obligations. | 0 / 2 / 0 = **2** | G64–67, 69 | **Remove the AI-specific restatement.** These obligations apply to all contributions. Link to the contributing guide; preserve any missing provenance requirement there rather than in agent rules. |
| Maintainers may reject low-effort AI work and escalate repeated abuse. | 0 / 1 / 0 = **1** | G82–85 | **Drop the AI-specific paragraph.** Judge quality and ownership through ordinary moderation. This changes the stated enforcement rationale; existing triage text is not an exact substitute for every provision. |
| Factual work suits AI; subjective work belongs to humans. | 0 / 1 / 0 = **1** | G15–22 | **Drop.** Too broad to guide action and muddied by PR Details requesting motivations and trade-offs. Concrete boundaries suffice. |

### Development

Keep the unusual constraints that prevent expensive mistakes.

| Claim | Occurrences: A / G / P | Evidence | Decision, destination, and brief wording |
| --- | --- | --- | --- |
| Validate Alloy configuration against real definitions. | 1 / 1 / 0 = **2** | A90–91; G73–75 | **Keep → A.** “Check Alloy configuration against this repo's component definitions.” |
| Run lint and relevant tests. | 1 / 0 / 0 = **1**, plus 3 examples | A92–94; examples A141–157 | **Keep once → A.** “Validate implementation changes with `make lint` and relevant tests; report anything you could not run.” Keep one focused-test example. |
| Dependency changes require collector regeneration. | 1 / 0 / 0 = **1** | A95–97 | **Keep → A.** “After changing `require` lines in any `go.mod`, run `make generate-otel-collector-distro`; include generated updates and verify a second run is clean.” |
| Changelogs are generated, not hand-edited. | 1 / 0 / 0 = **1** | A46 | **Keep → A.** “Do not edit changelogs; release tooling generates them.” |
| Keep PRs focused; split titles containing “and.” | 1 / 0 / 0 = **1** | A47–48 | **Drop the “and” test.** Keep “One logical change per PR” in the contributing guide. No conjunction-based heuristic. |
| Use project templates. | 1 / 1 / 1 = **3** | A53–57; G52; P22 | **Keep → A.** “Use the repository templates when preparing contributions.” Remove the other copies. |
| Follow Conventional Commit and capitalization rules. | 1 / 0 / 2 = **3**, plus A15–16 pointer | A66–72; P9–11, 25–27 | **Keep definition in contributing guide + existing CI.** A links to the guide for PR preparation. Remove copied syntax and examples. |
| Verify issue references rather than invent them. | 1 / 1 / 1 = **3** | A53–57; G32; P25–27 | **Drop separately.** An instance of factual accuracy, not a distinct contribution policy. Restore targeted wording if unsupported references are a recurring observed problem. |

### Navigation and environment

Delete unnecessary onboarding instead of automatically finding another home for it.

| Material | Repetition / location | Decision |
| --- | --- | --- |
| Repository summary and module list | A5–6 | **Drop.** Discoverable; the generation rule carries the important cross-module consequence. |
| Playbook catalogue | A12–21, 109–114, 121–131 | **Cut to a contributing-guide pointer and docs-specific pointer.** Tests appear twice, writing docs three times, component docs twice, breaking changes twice. Do not recreate the catalogue as a routing table. |
| Documentation style and role references | A109–111; Copilot and Cursor adapters | **Keep shared docs guidance.** Adapter references serve a loading purpose; they are not three separately maintained style guides. |
| Build/run/help commands | A135–168; skip-UI advice repeated at A182–183 | **Keep `make help`; drop the tutorial from A.** Setup details belong in ordinary developer documentation. |
| Cursor VM prerequisites and workarounds | Six bullets at A175–185 | **Remove from universal guidance.** Retain in actual environment setup only if still needed. Drop timing estimates and stale-version advice rather than automatically relocating them. |
| Tool entry points | `CLAUDE.md`, Copilot, Cursor | **Keep thin pointers.** No new rule framework or adapter project as a prerequisite for deciding policy. |

## What remains

Three small homes, containing only the surviving claims:

**`genai.md`: accountability and the public-conversation boundary.** Candidate core:

> AI assistance is welcome. You are responsible for everything you submit: understand and review
> it, whatever tools you use.
>
> Write submitted issues, proposals, review conclusions, and discussion yourself; do not have an
> agent draft or send them for you. Automated submissions require project approval.
>
> Follow the contribution guide and complete the PR checklist yourself, including AI disclosure.

**`AGENTS.md`: a policy pointer and operational exceptions.** Read `genai.md`; validate Alloy
configuration against source; run required checks; regenerate collector dependencies; leave
changelogs to release tooling; use templates. Add the contributing and docs-style pointers and
one useful test command. No ownership taxonomy, refusal script, playbook index, or VM handbook.

**PR template: fields and attestation.** Keep headings and field hints. Keep the checklist as a
human attestation, with one instruction to leave it unchanged when preparing a draft. Put the
disclosure threshold beside its checkbox. Remove the policy essay. Recommend removing the
reviewer-notes authorship restriction; if the team retains it, keep its field marker and one
sentence defining the marker without restoring all the explanations.

The material choices are whether reviewer notes need exclusive human authorship, whether private
brainstorming needs special warnings, and whether enforcement needs AI-specific language. My
recommendations are **no, no, and no**. These are proposed policy changes, not research findings
or changes already applied.

Next step: disagree with individual rows, then write only the survivors. A short manual exercise
with a code task, a PR draft, and a reviewer-reply request is sufficient for an initial sanity
check, not proof of reliability. The earlier evaluation framework is not a prerequisite for this
editorial decision.
