# Proposal: Alloy proposal process

* Author: Robert Fratto (@rfratto)
* Last updated: 2024-05-22
* Original issue: https://github.com/grafana/alloy/issues/908

## Abstract

Formalize a process for proposing changes to Alloy and reviewing proposed
changes.

## Problem

Today, there is no formalized process for proposing changes to Alloy, and no
formalized process for reviewing proposed changes. This has led to governance
members being unsure how to get a proposal reviewed. This becomes exacerbated
for external contributors who do not have a direct line of communication to
governance members. The result of a lack of process can be seen by the number
of open proposals in an unknown state; 47 at the time of writing.

At the same time, more and more proposals for Alloy have been performed
offline. This has gradually drifted away from the [design in the open][]
mentality previously accepted by Grafana Agent. This has made it more difficult
for external contributors to understand and influence the direction of the
Alloy project.

Formalizing a process for creating and reviewing proposals will help ensure that:

* All contributors can contribute to the direction of Alloy.
* All existing and new proposals are guaranteed to be evaluated and decided on.
* A publicly accessible single source of truth is used for discussing all
  proposals.
* Make it clear when a change to Alloy requires consensus and approval before
  code is acceptable.

[design in the open]: https://github.com/grafana/agent/blob/main/docs/rfcs/0001-designing-in-the-open.md

## Proposal

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

At a high-level, proposals go through the following stages:

1. *Issue*: A brief [issue][new-proposal] is created for the proposal.

2. *Discuss*: Public discussion on the proposal issue drives the proposal
   towards one of three outcomes:

   * Accept the proposal, or
   * decline the proposal, or
   * ask for a design document.

   As public discussion occurs, the proposal is expected to be refined over
   time to address incoming feedback.

   If the proposal is accepted or declined, the proposal process ends here.

3. *Design document*: If requested, a design document is written, and the PR
   for the design document becomes the new home for discussing the proposal.

4. *Consensus*: Once comments and changes have slowed down, a final public
   discussion aims to reach consensus for one of two outcomes:

   * Accept the proposal, or
   * decline the proposal.

[new-proposal]: https://github.com/grafana/alloy/issues/new?assignees=&labels=proposal&projects=&template=proposal.yaml

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

```markdown
# Proposal: [Title]

* Author(s): [List of proposal authors]
* Last updated: [Date]
* Original issue: https://github.com/grafana/alloy/issues/NNNN

## Abstract

[A short summary of the proposal.]

## Problem

[A description of the necessary context and the problem being solved by the proposed change. The problem statement should explain why a solution is necessary.]

## Proposal

[A description of the proposed change with a level of detail sufficient to evaluate the tradeoffs and alternatives.]

## Pros and cons

[A list of trade offs, advantages, and disadvantages introduced by the proposed solution.]

## Alternative solutions

[If applicable, a discussion of alternative approches. Alternative approaches should include pros and cons, and a rationale for why the alternative approach was not selected.]

## Compatibility

[A discussion of the change with regard to backwards compatibility.]

## Implementation

[A description of the steps in the implementation, who will do them, and when.]

## Related open issues

[If applicable, a discussion of issues relating to this proposal for which the author does not know the solution. This section may be omitted if there are none.]
```

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

A publicly available GitHub project tracks proposals and their states.

[governance]: https://github.com/grafana/alloy/blob/main/GOVERNANCE.md


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

## Pros and cons

Formalizing proposal creation and review ensures that all proposals are
eventually given attention. Having proposals reviewed weekly ensures that
proposals are kept moving forward and nobody is left waiting months for a
decision to be made.

To re-embrace the spirit of "design in the open," conversations about a
proposal should be discoverable by new parties to make it easy to participate.
Having a single source of truth for discussions (the proposal issue or design
document PR) prevents concurrent conversations from fragmenting in multiple
locations. This ensures that contributors know where to look for active
discussions and important information doesn't get lost while trying to reach
consensus.

However, this process comes with some trade-offs, described in sections below.

### Proposal lifecycle time

Formalizing the proposal process ensures that all proposals are given
attention. However, the process described here will slow down the acceptance
rate of proposals.

Because changing a proposal's state is tied to the frequency of proposal review
process, a weekly proposal review process means that the acceptance cycle of
most proposals is increased to take at least two or three weeks; up to one week
to move to _Active_, at least one week to move to _Likely Accept_, and at least
one week to move to _Accepted_. This does not include the time a proposal may
spend in _Hold_.

Three weeks is half the time of our release cadence, and so proposals opened
too late in the cycle may need to wait two releases before being available to
users.

However, if proposals are needed more urgently, the proposal review group can
fast-track proposals. This should mitigate the impact of this trade-off. If
this mitigation is not sufficient, further proposals can be explored to reduce
the time it takes to get a proposal approved and implemented into a release.

### Conversational toil

Using GitHub for all proposals ensures there is a single source of truth at a
given time for discussion, and that all interested parties can participate in
shaping the future of Alloy.

However, GitHub is not well-suited for long-form discussions. This may lead to
long threads that can be difficult to follow.

There are some mitigation strategies to help with this:

* Discussion on issues should be done conversationally rather than responding
  to individual lines of text. This can help reduce the total number of
  comments and make a conversation more digestible.

* Authors of proposals can start with offline discussions and use alternative
  tools (such as Google Docs) to prepare proposals for public presentation.

* The proposal review group keeps discussions focused by preventing derailing
  and pointing out when concurrent conversational threads can be joined
  together under a larger idea.

* More involved proposals are transformed into design document PRs, where
  tooling for discussions is better suited for long-form discussions. Less
  involved proposals can require less discussion, potentially avoiding the
  issue of an issue becoming long-winded.

## Compatibility

This proposal is purely procedural and does not affect the backward
compatibility of Alloy.

## Implementation

This proposal is written using the new process to serve as an example.
Following the proposed process, the next steps are:

1. The proposal is currently in the _Active_ state; it bypasses _Incoming_ as it
   is the only proposal using this process. Discussion towards a consensus
   (accept or decline) is ongoing at #909.

2. Once discussion slows down, the proposal will move to either _Likely Accept_
   or _Likely Decline_. This depends on when discussion slows down, but is
   likely to be at least a week from the date this was opened.

3. After being in _Likely Accept_ or _Likely Decline_ for a week, this proposal
   will be either formally accepted or declined and move to the relevant state.

If this proposal is accepted, all existing and new proposals will be subject to
the process described here, and the following will occur:

* A new public project board will be created for tracking proposals in their
  various states.

* A more general form of the proposal process will be moved to
  `docs/design/README.md`.

* Authors of existing proposals may be asked to make changes to their proposal
  to adhere to this new process, such as moving embedded design documents into
  a design document PR.

## Acknowledgments

Heavily inspired by [Go's proposal review process](https://github.com/golang/proposal)
and John Ousterhout's [Open Decision-Making](https://web.stanford.edu/~ouster/cgi-bin/decisions.php).
