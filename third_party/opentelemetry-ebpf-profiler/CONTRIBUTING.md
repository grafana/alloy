# Contributing to opentelemetry-ebpf-profiler

The Profiling special interest group (SIG) meets regularly. See the
OpenTelemetry
[community](https://github.com/open-telemetry/community)
repo for information on this and other SIGs.

## Community

See the [public meeting
notes](https://docs.google.com/document/d/19UqPPPlGE83N37MhS93uRlxsP1_wGxQ33Qv6CDHaEp0/edit#heading=h.4rdgawyis2hd)
for a summary description of past meetings.

See the [calendar
group](https://groups.google.com/a/opentelemetry.io/g/calendar-profiling) to
get invited to meetings.

See the [#otel-profiles](https://cloud-native.slack.com/archives/C03J794L0BV)
slack channel for discussions and questions.

## Pre-requisites

- Linux (5.4+ for x86-64, 5.5+ for ARM64) with eBPF enabled (the profiler currently only runs on Linux)
- Go as specified in [go.mod](https://github.com/open-telemetry/opentelemetry-ebpf-profiler/blob/main/go.mod)
- docker
- Rust as specified in [Cargo.toml](https://github.com/open-telemetry/opentelemetry-ebpf-profiler/blob/main/Cargo.toml)

## Development

You can view and edit the source code by cloning this repository:

```sh
git clone https://github.com/open-telemetry/opentelemetry-ebpf-profiler
```

Run `make test` to run the tests instead of `go test`.


## Pull Requests

### How to Send Pull Requests

Everyone is welcome to contribute code to `opentelemetry-ebpf-profiler` via
GitHub pull requests (PRs).

To create a new PR, fork the project in GitHub and clone the upstream
repo:

```sh
git clone https://github.com/open-telemetry/opentelemetry-ebpf-profiler
```

This will put the project in `opentelemetry-ebpf-profiler` in
current working directory.

Enter the newly created directory and add your fork as a new remote:

```sh
git remote add <YOUR_FORK> git@github.com:<YOUR_GITHUB_USERNAME>/opentelemetry-ebpf-profiler
```

Check out a new branch, make modifications, run linters and tests, and push the
branch to your fork:

```sh
git checkout -b <YOUR_BRANCH_NAME>
# edit files
# update changelog
git add -p
git commit
git push <YOUR_FORK> <YOUR_BRANCH_NAME>
```

Open a pull request against the main `opentelemetry-ebpf-profiler` repo.

Avoid rebasing and force-pushing to your branch to facilitate reviewing the
pull request.
Rewriting Git history makes it difficult to keep track of iterations during
code review.
All pull requests are squashed to a single commit upon merge to `main`.

### How to Receive Comments

* If the PR is not ready for review, please put `[WIP]` in the title,
	tag it as `work-in-progress`, or mark it as
	[`draft`](https://github.blog/2019-02-14-introducing-draft-pull-requests/).
* Make sure CLA is signed and CI is clear.

### How to Get PRs Merged

A PR is considered **ready to merge** when:

* It has received two qualified approvals[^1].

	This is not enforced through automation, but needs to be validated by the
	maintainer merging.
	* PRs introducing changes that have already been discussed and consensus
		reached only need one qualified approval. The discussion and resolution
		needs to be linked to the PR.

* All feedback has been addressed.
	* All PR comments and suggestions are resolved.
	* All GitHub Pull Request reviews with a status of "Request changes" have
		been addressed. Another review by the objecting reviewer with a different
		status can be submitted to clear the original review, or the review can be
		dismissed by a [Maintainer] when the issues from the original review have
		been addressed.
	* Any comments or reviews that cannot be resolved between the PR author and
		reviewers can be submitted to the community [Approver]s and [Maintainer]s
		during the weekly SIG meeting. If consensus is reached among the
		[Approver]s and [Maintainer]s during the SIG meeting the objections to the
		PR may be dismissed or resolved or the PR closed by a [Maintainer].
	* Any substantive changes to the PR require existing Approval reviews be
		cleared unless the approver explicitly states that their approval persists
		across changes. This includes changes resulting from other feedback.
		[Approver]s and [Maintainer]s can help in clearing reviews and they should
		be consulted if there are any questions.

* The PR branch is up to date with the base branch it is merging into.
	* To ensure this does not block the PR, it should be configured to allow
		maintainers to update it.

* It has been open for review for at least one working day. This gives people
	reasonable time to review.

* All required GitHub workflows have succeeded.
* Urgent fix can take exception as long as it has been actively communicated
	among [Maintainer]s.

Any [Maintainer] can merge the PR once the above criteria have been met.

[^1]: A qualified approval is a GitHub Pull Request review with "Approve"
	status from an OpenTelemetry Profiler [Approver] or [Maintainer].

## Approvers and Maintainers

### Maintainers

- [Christos Kalkanis](https://github.com/christos68k), Elastic
- [Dmitry Filimonov](https://github.com/petethepig), Pyroscope/Grafana
- [Felix Geisendörfer](https://github.com/felixge), Datadog
- [Timo Teräs](https://github.com/fabled)

For more information about the maintainer role, see the [community repository](https://github.com/open-telemetry/community/blob/main/guides/contributor/membership.md#maintainer).

### Approvers

- [Damien Mathieu](https://github.com/dmathieu), Elastic
- [Florian Lehner](https://github.com/florianl), Elastic
- [Joel Höner](https://github.com/athre0z)
- [Tim Rühsen](https://github.com/rockdaboot), Elastic

For more information about the approver role, see the [community repository](https://github.com/open-telemetry/community/blob/main/guides/contributor/membership.md#approver).

### Become an Approver or a Maintainer

See the [community membership document in OpenTelemetry community
repo](https://github.com/open-telemetry/community/blob/main/guides/contributor/membership.md).

[Approver]: #approvers
[Maintainer]: #maintainers
