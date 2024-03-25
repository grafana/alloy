# Designing in the Open

* Date: 2021-11-02
* Author: Robert Fratto (@rfratto)
* PR: [grafana/agent#1055](https://github.com/grafana/agent/pull/1055)

## Summary

Many open source projects start behind closed doors, where it's designed,
prototyped, and tested before being released publicly. This can be true
regardless of why the project is being made; even personal side projects likely
start by someone designing alone.

Meanwhile, many open source projects might want to create a community of
developers. Much of the beauty of successful open source projects originates
from the varied backgrounds of its contributors: different people with
different use cases combining together to make a widely useful piece of
software.

However, even with an intent to foster a community of developers, it's natural
to accidentally build a habit from the closed-door design process. Even when
once-private proposals are made public, potential external contributors can
find themselves simply as spectators:

* Initial design is gated to core maintainers, in particular if they all work
  for the same company. This leaves less room for new people to help out.
* New concerns are less impactful if the proposal already received core
  maintainer consensus.
* Historical proposals with context and discussions become hard to find.

I believe it takes a deliberate inversion of process to foster community
participation. This document proposes how Grafana Agent will utilize public
spaces for its primary home for future design proposals.

## Goals

* Outline options for proposing changes to Grafana Agent
* Lower the barrier to entry for interested parties to become maintainers

## Non-Goals

* Enforce that every change originates from a fully public proposal or
  discussion. While all maintainers and contributors will be encouraged to
  design openly, there may be legal, security, privacy, or business reasons
  that prevent some or all context from being made public.

* Be overly prescriptive: too many rules can hinder adoption of a process. This
  document outlines intention, not hard policy.

## Proposal

Public proposals may take one of two forms:

* Issue proposals
* RFC PR proposals (e.g., this document)

### Issues

Issues are the quickest path towards proposing a change. Issue proposals must
be opened at the [issues page](https://github.com/grafana/alloy/issues).

There are no strict set of rules for issue-based proposals, but authors are
recommended to prefix the issue title with `Proposal:` so it may be found more
easily.

### RFC PRs

RFC PR proposals must at least:

* Be placed in the `docs/rfcs` folder of the `grafana/agent` repository
* Have a lowercase filename in hyphen-case with an `.md` extension
* Prefix the filename with the RFC ID
  * ID `xxxx` may be initially used until the final ID is known
* Contain valid markdown
* Start with the title of the proposal
* Contain a bullet point list of metadata of:
  * The date the proposal was written
  * The list of authors, with their names and GitHub usernames
  * The PR where the proposal was posted
  * The status of the proposal

`0000-template.md` contains a template to use for writing proposals that
conforms to these rules.

The remainder of the proposal may be formatted however the author wishes. Some
example sections in the RFC may be:

* Summary: What is the background that lead to this proposal?
* Goals: What are the main goals of the proposal?
* Non-Goals: What _aren't_ the main goals of the proposal?
* Proposal: What is the proposal?
* Pros/Cons:
  * Pros: What are the upsides to this proposal?
  * Cons: What are the downsides to this proposal?
* Considered Alternatives: Why is this proposal the best path forward? What
  were the alternatives?
* Open Questions: What questions still need to be answered?
* Prior Art: What was this proposal based on, if anything?

#### RFC Status

The "Status" field of an RFC must be one of the following:

* Draft: This RFC is a work-in-progress and may change
* Implemented: Relevant code for this RFC has been merged to the main branch
* Deprecated: This RFC is no longer relevant to the current state of the
  project

RFCs may be merged in Draft state as work on them progresses. The _Draft_ state
is intended to signal to readers that an RFC is in flux. Once all relevant code
for an RFC is merged to main, the RFC may move to the _Implemented_ status.
RFCs without code, such as this RFC, may immediately be set as Implemented.

If, for any reason, an RFC becomes no longer relevant (deprecated by another
RFC, code removed, etc.), its status should move to Deprecated.

#### RFC Review

RFCs should be opened as a PR to grafana/agent, ideally prefixed in the PR
title with `RFC:` to easily identify it amongst other PRs.

### Google Docs Proposals

Google Docs may be useful for early feedback rounds during a proposal. However,
it is not recommended for the permanent home of a proposal:

* Change and comment history may not be available to all viewers.

* The file owner may delete the proposal, leading to a gap in historical
  context.

Google Docs proposals will be permitted if linked to from an issue proposal.
Google Doc proposals must be converted into an RFC proposal prior to formally
accepting the proposal. Enforcing this ensures that historical context is
recorded, though it is still not ideal as it discards comment history.

## Accepting Proposals

All readers are encouraged to engage in reviewing proposals. However, whether a
proposal is accepted is determined by [rough consensus][] of the Grafana Agent
governance team. External contributors may eventually be invited to [join the
governance team][governance] if they have a history of making ongoing
contributions to the project or community.

## Considered alternatives

A few existing public proposal processes have been examined for inspiration:

* [IETF's RFCs](https://www.ietf.org/standards/rfcs/)
* [Rust's RFCs](https://github.com/rust-lang/rfcs)
* [Joyent's Requests for Discussions](https://github.com/joyent/rfd)
* [OpenTelemetry's OTEPs](https://github.com/open-telemetry/oteps)
* [Kubernetes Enhancement Proposals (KEPs)](https://github.com/kubernetes/enhancements)

All of these processes are similar, but in the end, the current objective is to
start collecting proposals publicly rather than to be prescriptive yet.

[rough consensus]: https://github.com/grafana/alloy/blob/main/GOVERNANCE.md#technical-decisions
[governance]: https://github.com/grafana/alloy/blob/main/GOVERNANCE.md#team-members
