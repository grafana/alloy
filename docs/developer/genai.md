# Generative AI Contribution Policy

This page is intended to be read by **human contributors**.

Generative AI tools can help you contribute to Grafana Alloy. This page explains what we expect when
you use AI assistance.

## Core principle

**Humans propose changes and remain accountable. AI may assist with implementation.**

By submitting a contribution, you vouch for it as your own work. You are expected to understand it,
review it, and defend it in discussion, regardless of which tools helped you write the code.

Contribution work has two sides.

- **Factual and mechanical** work (what changed, implementing code or docs from clear intent, a
  brief PR summary) is where AI is often well suited.
- **Subjective and human** work (motivations, trade-offs, reviewer notes, and discussion) stays with
  you. Use AI where it helps; keep human oversight and interaction where judgment and accountability
  matter.

## Ownership model

| You own (write yourself)                   | AI may help with                              |
| ------------------------------------------ | --------------------------------------------- |
| Intent and design of the change            | Exploring the codebase                        |
| Pull request details (why, how, decisions) | Writing or refactoring code and tests         |
| Issue and proposal text you submit         | Editing documentation files in the repo       |
| Review replies and ongoing discussion      | Brief description of the PR (factual summary) |
| Code review conclusions                    | Issue(s) fixed (`Fixes #N` when known)        |
| Conventional Commit PR title               |                                               |

AI-assisted **implementation** is welcome when you review and understand the result. AI-mediated
**conversation** is not. Reviewers need confidence that you understand the design choices and
trade-offs of what you're proposing.

## Acceptable

- Use AI to implement or refactor code and docs that you then review and refine.
- Use AI to draft the PR **Brief description** and **Issue(s) fixed** from the actual change.
- Use AI to learn the codebase before contributing or reviewing.
- Use AI privately to clarify your own thinking, then post discussion **in your own words**.

## Not acceptable

- Leave pull request details empty of your own reasoning on non-trivial PRs, or paste AI text there
  as a stand-in for how/why you built the change.
- Paste AI-generated text into review threads or issue discussion as a stand-in for your analysis.
- Wire an agent to reply to reviewers on your behalf.
- Use AI as a substitute for your judgment when **reviewing** others' code.
- File automated, bot-driven issues or PRs from tools the Alloy team has not approved.
- Submit PRs or issues that ignore the project templates.

Approved bots (for example dependency bots) are fine when clearly marked as bot-generated.

## Disclosure

When AI generates the **bulk of the implementation**, check **"This pull request was substantially
generated with AI assistance"** in the PR template. Minor autocomplete or small edits do not need
disclosure.

An AI-written Brief description and Issue(s) fixed lines are fine. Disclosure does not excuse
AI-written pull request details or discussion — those must still be yours.

## Licensing and provenance

Alloy is [Apache 2.0][license]. Contributions require a signed [CLA][cla]. The same obligations
apply to AI-assisted work: you warrant that you have the right to contribute it, that it is
license-compatible, and that any new dependencies follow the [dependency
guidance][contributing-deps].

If you are unsure whether generated code is original or license-compatible, do not submit it.

## Alloy tip

LLMs often invent Alloy component names, arguments, and config syntax. Point tools at the [Alloy
docs][alloy-docs] and the relevant source in this repo, and validate output against real component
definitions and project checks before submitting.

For new components and larger changes, follow the [proposal process][proposal-process]. AI may help
you draft; you own the argument in public discussion.

## Enforcement

Maintainers may close or request changes on contributions where pull request details or discussion
look unowned or low-effort AI-generated, using the same [issue triage][issue-triage] process as
other contributions. Repeated abuse may lead to stricter review or blocked contributions.

[alloy-docs]: https://grafana.com/docs/alloy/latest/
[issue-triage]: ./issue-triage.md
[license]: ../../LICENSE
[cla]: https://cla-assistant.io/grafana/alloy
[contributing-deps]: ./contributing.md#dependency-management
[proposal-process]: ../design/README.md
