# Alloy proposals

## Introduction

Grafana Alloy has a formalized procedure for proposing changes to Alloy.

The proposal process is intended for user-facing features, enhancements, or any
largely scoped internal change. Other changes typically do not need a proposal,
such as minor code refactors.

If a contributor is uncertain about whether a proposal is needed, it is
preferable to create a proposal instead of jumping into a change that may be
declined.

## Process

Proposals are the process of getting consensus on a problem and a solution to
that problem. Proposals are the more detailed form of an enhancement request,
which typically only include a problem statement but no suggested solution.

Proposals typically include:

* A clear problem statement.
* A proposed solution to the problem statement.
* If applicable, a list of alternative solutions with rationale for why the
  alternative solutions were not selected.

Proposals can still meet the above criteria while remaining brief. For example,
this proposal would be sufficient:

> I would like to use the batch processor component from OpenTelemetry
> Collector in Grafana Alloy to reduce the number of outgoing network requests.
> Please add the batch processor as a new Alloy component called
> `otelcol.processor.batch`.

Any existing issue can be turned into a proposal by adding a `proposal` label.

At a high-level, proposals go through the following stages:

1. **Issue**: A brief [issue][new-proposal] is created for the proposal. At
   this stage, there is no need for a design document.

2. **Discuss**: Public discussion on the proposal issue drives the proposal
   towards one of three outcomes:

   * Accept the proposal, or
   * decline the proposal, or
   * ask for a design document.

   As public discussion occurs, the proposal is expected to be refined over
   time to address incoming feedback.

3. **Design document**: If requested, a design document is written, and the PR
   for the design document becomes the new home for discussing the proposal.

4. **Consensus**: Once comments and changes have slowed down, a final public
   discussion aims to reach consensus for one of two outcomes:

   * Accept the proposal, or
   * decline the proposal.

[new-proposal]: https://github.com/grafana/alloy/issues/new?assignees=&labels=proposal&projects=&template=proposal.yaml

## Detail

### Scope

The proposal process is intended for user-facing features, enhancements or any
largely scoped internal change. Other changes typically do not need a proposal,
such as minor code refactors. If a contributor is uncertain about whether a
proposal is needed, it is preferable to create a proposal instead of jumping
into a change that may be declined.

Proposals may initially be brief, with just enough detail to explain what is
being proposed, why it is being proposed, and any relevant details that may
guide the public discussion towards consensus.

Proposal authors may skip straight to creating a design document for their
proposal if they know one will be needed. If in doubt, it's better to create a
proposal issue first.

Any existing issue can be turned into a proposal by adding a `proposal` label.

### Compatibility

Proposals that break [backward compatibility] are moved to a "Hold" state until
work on the next major release begins. As major releases are infrequent, this
avoids approved proposals from becoming obsolete before they are implemented.

[backward compatibility]: https://grafana.com/docs/alloy/latest/introduction/backward-compatibility/

### Design documents

Proposals that require more careful consideration and explanation require a
design document.

* Design documents should be checked into the Alloy repository as
  `docs/design/NNNN-name.md`, where `NNNN` is the GitHub issue number of the
  proposal and `name` is a short descriptive name deliminated by hyphens.

* Design documents should follow the template in `docs/design/template.md`.

* Design documents should address any specific concerns raised during the
  initial discussion.

* Design documents should acknowledge and address alternative solutions, if
  known, to the problem statement.

Once a design document is created, the proposal author should:

* Close the original proposal issue and link to the design document PR.
* Update the proposal issue description with a link to the design document.

Closing the original proposal issue in favor of the design document PR prevents
fragmenting discussions as contributors work towards consensus on a proposal.

The state of the design document PR is used to denote whether the proposal is
accepted or declined and is treated as the source of truth. The design document
content should not include any indication of the current status of the
proposal.

The PR for design documents are merged once the associated proposal becomes
_Accepted_. If the proposal becomes _Declined_, the PR for the design document
is closed.

### Design document template

The design document template can be found at [./template.md](./template.md).

### Proposal review

Every week, a subset of Alloy [governance] members review open and pending
proposals as part of the proposal review process. The proposal review process
may be performed asynchronously or synchronously, up to the discretion of the
governance members.

The goal of proposal review is to make sure proposals are moving forward and
are receiving attention from the right people. Governance members participating
in proposal review should cc relevant maintainers, raise important questions,
ping on stale discussions, and generally drive discussion towards some kind of
consensus.

The proposal review process also identifies when consensus has been reached on
a proposal and moves that proposal to the next stage.

The Alloy governance team can, at their discretion, make exceptions that do not
need to go through all stages, fast-tracking them to any other stage. This may
be appropriate for proposals that do not merit the full review process or
proposals that need to be considered quickly, such as an upcoming release
deadline.

Proposals are in one of the following states:

* **Incoming**: New proposals are given the _Incoming_ state.

  The proposal review process prioritizes proposals in other states to keep
  things moving forward. If time permits during the review process, proposals
  in _Incoming_ are moved to _Active_.

* **Active**: Proposals in the _Active_ state are reviewed during the proposal
  review process to watch for emerging consensus in the discussion. The
  proposal review group may also comment, make suggestions, ask clarifying
  questions, and try to restate the proposal to make sure everyone agrees about
  what exactly is being discussed.

* **Likely Accept**: If a proposal discussion seems to have reached a consensus
  to accept a proposal, the proposal review group moves the proposal to the
  _Likely Accept_ state. This change should be indicated with a comment on the
  proposal. This state marks the final period for comments that might change
  the recognition of consensus.

* **Likely Decline**: If a proposal discussion seems to have reached a consensus
  to decline a proposal, the proposal review group moves the proposal to the
  _Likely Decline_ state.

  A proposal may also be moved to _Likely Decline_ if the proposal review group
  identifies that no consensus is likely to be reached and that the default of
  not accepting the proposal is appropriate.

  Similarly to _Likely Accept_, this change should be indicated with a comment
  on the proposal, and this state marks the final period for comments that
  might change the recognition of consensus.

* **Accepted**: If a proposal has been in _Likely Accept_ for a week and
  consensus has not changed, the proposal is moved to the _Accepted_ state.

  If significant discussion happens during the _Likely Accept_ period, the
  proposal review group may decide to leave the proposal in _Likely Accept_ for
  another week or even move the proposal back to _Active_.

  Once a proposal has been Accepted, implementation work may begin.

* **Declined**: If a proposal has been in _Likely Decline_ for a week and
  consensus has not changed, the proposal is moved to the _Declined_ state.

  If significant discussion happens during the _Likely Decline_ period, the
  proposal review group may decide to leave the proposal in _Likely Decline_
  for another week or even move the proposal back to _Active._

  If a proposal is declined, other proposals may be opened to address the same
  problem with a different approach. Re-opening identical proposals are likely
  to be set as _Declined as Duplicate_ unless there is significant new
  information.

* **Declined as Duplicate**: If a proposal is a duplicate of a previously
  decided proposal, the proposal review group may decline the proposal directly
  without progressing through other stages.

  Reconsidering a previously declined proposal with the same approach may be
  considered when there is significant new information.

* **Declined as Infeasible**: If a proposal directly contradicts the core
  design of Alloy, or if a proposal is impossible to implement efficiently or
  at all, the proposal review group may decline the proposal as infeasible
  without progressing through other stages.

  If it seems like there is still general interest from others, or that
  discussion may lead to a feasible proposal, the proposal may be kept open and
  the discussion continued.

* **Declined as Retracted**: If a proposal is closed or retracted in a comment
  by the original author, the proposal review group may decline the proposal as
  retracted without progressing through other stages.

  If the proposal is retracted in a comment and there is still general interest
  from others, the proposal may be kept open and the discussion continued.

* **Declined as Obsolete**: If a proposal is obsoleted by changes to Alloy that
  have been made since the proposal was filed, the proposal review group may
  decline the proposal as obsolete without progressing through other stages.

  If it seems like there is still general interest from others, or that
  discussion may lead to a different, non-obsolete proposal, the proposal may
  be kept open and the discussion continued.

* **Hold**: If a discussion of a proposal requires design revisions or
  additional information that will not be available for a couple of weeks or
  more, the proposal review group moves the proposal to the _Hold_ state with a
  note of what it is waiting on. Once the proposal is unblocked, the proposal
  can be moved back to the _Active_ state for consideration during the next
  proposal review.

A [publicly available GitHub project][project] tracks proposals and their states.

[governance]: https://github.com/grafana/alloy/blob/main/GOVERNANCE.md
[project]: https://github.com/orgs/grafana/projects/663

### Consensus and disagreement

The goal of the proposal process is to reach general public consensus about the
outcome in a timely manner.

General public consensus does not require unanimity; one person disagreeing
will not necessarily prevent consensus from being achieved. Consensus should
consider all raised concerns, but not necessarily addressing all concerns
raised.

If the proposal review process cannot identify a general consensus of the
proposal, the usual result is that the proposal is declined. It can happen that
the proposal review process may not identify a general consensus but the
proposal should not be outright declined. For example, there may be a consensus
that a solution to a problem is important, but not which solution should be
adopted.

If the proposal review group cannot identify a consensus nor a next step to
reach consensus, the decision about the path forward is moved to the Alloy
[governance] team. The governance team will discuss the proposal offline and
aim for consensus. If consensus is reached, the outcome and rationale will be
documented and the proposal will move to the appropriate state.

If the governance team cannot reach consensus, the path forward is called to a
formal governance [vote][] as a last resort. The result of the vote is
documented and the proposal will move to the appropriate state.

[governance]: https://github.com/grafana/alloy/blob/main/GOVERNANCE.md
[vote]: https://github.com/grafana/alloy/blob/main/GOVERNANCE.md#voting
