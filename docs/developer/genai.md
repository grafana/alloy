# Generative AI Contribution Policy

Generative AI (GenAI) tools such as large language model (LLM) assistants can help you write code,
documentation, and proposals for Grafana Alloy. This policy explains what is an acceptable use of GenAI tools
when contributing to the project.

## Core principle

**The human contributor is always in control and fully responsible for their contribution.**

GenAI is a tool that assists you. It is not a substitute for your own understanding, judgment, or
accountability. By submitting a contribution, you are vouching for it as your own work, regardless
of which tools helped you produce it. You are expected to understand, review, and provide rationale for
everything you submit.

## Acceptable use

You are welcome to use GenAI tools to:

- Write or refactor code and documentation, as long as you actively review and refine the output.
- Understand the Alloy codebase, a component, or a subsystem before contributing or reviewing.
- Draft issues, proposals, or design documents that you then verify and shape into your own
  reasoning.
- Translate, summarize, or improve the clarity of text you have written.

In all cases, you remain the author: you read the output, you correct it, and you ensure your contribution meets the project's standards before submitting an issue, pull request, or comment.

## Not acceptable

Do not:

- Submit unreviewed, bulk AI-generated content to pull requests, issues, or proposals. If you
  did not read and understand it, do not submit it.
- Use GenAI as a substitute for human judgment in code review. An AI tool may help you understand a
  change, but the review and its conclusions must be yours.
- File automated, bot-driven issues or pull requests from tools that have not been approved by the
  Alloy team. This policy covers humans using AI assistance, not autonomous agents acting on their
  own.
- Paste generated text into a discussion without adding your own analysis and context.
- Wire up an AI agent to respond automatically to reviewers or other community members on your
  behalf. If you want help from GenAI in a discussion, use it yourself and post in your own words,
  rather than connecting it directly to maintainers.
- Submit PRs or issues which do not follow our defined templates
Maintainers may close low-effort AI-generated contributions, following the same
[issue triage process][issue-triage] used for other contributions. When they do, they should explain
why and, where appropriate, offer guidance on how to improve the contribution. Repeated low-effort
submissions will trigger additional review and offending users blocked from further contributing.

## Approved automation

Some automated tools are explicitly approved by the Alloy team and are an acceptable exception to the
rule against bot-driven contributions. Examples include GitHub Copilot and the automated dependency
review bot, and the Alloy team may approve more over time. Contributions from these tools are always
clearly marked as bot-generated so that reviewers can tell them apart from human contributions.

## Disclosure

When GenAI generates the **bulk** of a contribution, disclose it. For pull requests, check the
**"This pull request was substantially generated with AI assistance"** box in the pull request
template. Minor, incidental help such as autocomplete or small edits does not need to be disclosed.

Disclosure is not a mark against your contribution. It helps reviewers calibrate their attention and
keeps the project's expectations transparent.

## Licensing and provenance

Grafana Alloy is licensed under [Apache 2.0][license], and all contributions require a signed
[Contributor License Agreement (CLA)][cla]. These obligations apply equally to AI-assisted
contributions.

When you contribute AI-assisted code or content, you warrant that:

- You have the right to contribute it under the project's license and CLA.
- It does not reproduce code or text from sources whose licenses are incompatible with Apache 2.0
  (for example, GPL-licensed or proprietary code that an LLM may have reproduced from its training
  data).
- Any third-party code or dependencies it introduces are license-compatible and follow the
  [dependency guidance][contributing-deps] in the contributing guide.

Because Alloy vendors and wraps many upstream components, provenance matters. If you are unsure
whether generated code is original or where it came from, do not submit it.

## Alloy-specific guidance

LLMs frequently hallucinate Alloy configuration syntax, component names, and component arguments,
and may produce configurations or components that do not match the current schemas. To reduce these
errors, point the tool at authoritative sources rather than relying on its training data: provide the
[Alloy documentation][alloy-docs] and the relevant component source code, ideally pinned to the
specific Alloy version or git tag you are targeting, and ask the tool to validate its output against
them. Always validate AI-generated code and configuration against the real component definitions and
the project's checks before submitting.

For new components and larger changes, follow the existing [proposal process][proposal-process].
A proposal or design document must reflect your own reasoning. AI may help you draft it, but you own
the argument in the public consensus discussion.

[alloy-docs]: https://grafana.com/docs/alloy/latest/
[issue-triage]: ./issue-triage.md
[license]: ../../LICENSE
[cla]: https://cla-assistant.io/grafana/alloy
[contributing-deps]: ./contributing.md#dependency-management
[proposal-process]: ../design/README.md
