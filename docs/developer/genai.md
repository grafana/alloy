# Generative AI Contribution Policy

This page is intended to be read by **human contributors**.

Generative AI tools can help you contribute to Grafana Alloy. This page explains what we expect when
you use AI assistance.

## Core principle

**Humans propose changes and remain accountable. AI may assist with implementation.**

By submitting a contribution, you vouch for it as your own work. You are expected to understand it,
review it, and defend it in discussion, regardless of which tools helped you write the code.

Contribution work has two sides.

- **Factual and mechanical** work (what changed, implementing code or docs from clear intent, and
  writing a PR title and description sections not marked `HUMAN ONLY`) is where AI is often well
  suited.
- **Subjective and human** work (setting intent, PR sections marked `HUMAN ONLY`, and discussion)
  stays with you. Use AI where it helps; keep human oversight and interaction where judgment and
  accountability matter.

## Ownership model

| You own (write yourself)                         | AI may help with                           |
| ------------------------------------------------ | ------------------------------------------ |
| Intent and design of the change                  | Exploring the codebase                     |
| PR description sections marked `HUMAN ONLY`      | Writing or refactoring code and tests      |
| Issue and proposal text you submit               | Editing documentation files in the repo    |
| Review replies and ongoing discussion            | PR title and PR details summaries          |
| Code review conclusions                          | Issue(s) fixed (after verifying it exists) |

AI-assisted **implementation** is welcome when you review and understand the result. AI-mediated
**conversation** is not. Reviewers need confidence that you understand the design choices and
trade-offs of what you're proposing.

## Acceptable

- Use AI to implement or refactor code and docs that you then review and refine.
- Use AI to write the PR title and sections not marked `HUMAN ONLY` in the PR description.
- Use AI to learn the codebase before contributing or reviewing.
- Use AI privately to clarify your own thinking, then post discussion **in your own words**.

## Not acceptable

- Submit AI-written PR content that you don't understand, haven't reviewed, or can't defend.
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

Maintainers may close or request changes on contributions where discussion looks unowned or
low-effort AI-generated, using the same [issue triage][issue-triage] process as other contributions.
When they do, they will explain why and, where appropriate, offer guidance on how to improve the
contribution. Repeated abuse may lead to stricter review or blocked contributions.

[alloy-docs]: https://grafana.com/docs/alloy/latest/
[issue-triage]: ./issue-triage.md
[license]: ../../LICENSE
[cla]: https://cla-assistant.io/grafana/alloy
[contributing-deps]: ./contributing.md#dependency-management
[proposal-process]: ../design/README.md
