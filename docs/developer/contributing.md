# Contributing

Grafana Alloy uses GitHub to manage reviews of pull requests.

If you're planning to do a large amount of work, you should discuss your ideas in an
[issue][new-issue]. This will help you avoid unnecessary work and surely give you and us a good deal
of inspiration.

For trivial fixes or improvements, pull requests can be opened immediately without an issue.

## Before Contributing

- Review the following code coding style guidelines:
  - [Go Code Review Comments][code-review-comments]
  - The _Formatting and style_ section of Peter Bourgon's [Go: Best Practices for Production
    Environments][best-practices]
  - The [Uber Go Style Guide][uber-style-guide]
- Sign our [CLA][], otherwise we're not able to accept contributions.

## Steps to Contribute

Should you wish to work on an issue, please claim it first by commenting on the GitHub issue that
you want to work on it. This is to prevent duplicated efforts from contributors on the same issue.

Please check the [`good first issue`][good-first-issue] label to find issues that are good for
getting started. If you have questions about one of the issues, with or without the tag, please
comment on them and one of the maintainers will clarify it. For a quicker response, contact us in
the `#alloy` channel in our [community Slack][community-slack].

See next section for detailed instructions to compile the project. For quickly compiling and testing
your changes do:

```bash
# For building:
go build .
./alloy run <CONFIG_FILE>

# For testing:
make lint test # Make sure all the tests pass before you commit and push :)
```

We use [`golangci-lint`](https://github.com/golangci/golangci-lint) for linting the code.

As a last resort, if linting reports an issue and you think that the warning needs to be disregarded
or is a false-positive, you can add a special comment `//nolint:linter1[,linter2,...]` before the
offending line.

All our issues are regularly tagged with labels so that you can also filter down the issues
involving the components you want to work on.

## Compiling Alloy

To build Alloy from source code, please install the following tools:

1. [Git](https://git-scm.com/)
2. [Go](https://golang.org/) (see `go.mod` for what version of Go is required)
3. [Make](https://www.gnu.org/software/make/)
4. [Docker](https://www.docker.com/)

> **NOTE**: `go install` cannot be used to install Alloy due to `replace` directives in our `go.mod`
> file.

To compile Alloy, clone the repository and build using `make alloy`:

```bash
$ git clone https://github.com/grafana/alloy.git
$ cd alloy
$ make alloy
$ ./build/alloy run <CONFIG_FILE>
```

An example of the above configuration file can be found [here][example-config].

Run `make help` for a description of all available Make targets.

### Compile on Linux

Compiling Alloy on Linux requires extra dependencies:

- [systemd headers](https://packages.debian.org/sid/libsystemd-dev) for Loki components.

  - Can be installed on Debian-based distributions with:

    ```bash
    sudo apt-get install libsystemd-dev
    ```

### Compile on Windows

Compiling Alloy on Windows requires extra dependencies:

- [tdm-gcc](https://jmeubank.github.io/tdm-gcc/download/) full 64-bit install for compiling C
  dependencies.

## Pull Request Checklist

Changes should be branched off of the `main` branch. It's recommended to rebase on top of `main`
before submitting the pull request to fix any merge conflicts that may have appeared during
development.

PRs should not introduce regressions or introduce any critical bugs. If your PR isn't covered by
existing tests, some tests should be added to validate the new code (100% code coverage is _not_ a
requirement). Smaller PRs are more likely to be reviewed faster and easier to validate for
correctness; consider splitting up your work across multiple PRs if making a significant
contribution.

If your PR is not getting reviewed or you need a specific person to review it, you can @-reply a
reviewer asking for a review in the pull request or a comment, or you can ask for a review on the
Slack channel [#alloy](https://slack.grafana.com).

## Pull request titles and commit messages

We use [Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/) as the basis for
[CHANGELOG](../../CHANGELOG.md) entries, but **you don't need** to worry about this for anything
other than the Pull Request title. This is because we use squash commits when merging Pull Requests,
so the commit title and body are determined at merge time.

To ensure your Pull Request gets a proper changelog entry and semantic version bump, your **PR
title** must adhere to Conventional Commit style.

When a maintainer goes to merge your PR, the prompt they get will contain the PR title as the squash
commit's title and all of the individual commit details as the squashed commit's body.

> **NOTE TO MAINTAINERS:** PR merge time is the point at which you can modify the commit details to
> get the desired changelog entry, should it be needed. This can also be fixed after the fact, but
> it's easiest to address it at this point.

PR titles (and by extension CHANGELOG entries) should:

1. Read as a complete sentence in the imperative, present tense (e.g. "change", not "changes" or
   "changed").
2. Describe the impact on the user which is reading the changelog.

For example: `feat: increase config file read speed by 1500%`

> Readers should be able to understand how a change impacts them. Default to being explicit over
> vague.
>
> - Vague: `fix: fix issue with metric names`
> - Explicit: `fix: fix 's' getting replaced by 'z' in metric names`

## Dependency management

Alloy uses [Go modules][go-modules] to manage dependencies on external packages.

To add or update a new dependency, use the `go get` command:

```bash
# Pick the latest tagged release.
go get example.com/some/module/pkg@latest

# Pick a specific version.
go get example.com/some/module/pkg@vX.Y.Z
```

Tidy up the `go.mod` and `go.sum` files:

```bash
go mod tidy
```

You have to commit the changes to `go.mod` and `go.sum` before submitting the pull request.

### Using forks

Using a fork to pull in custom changes must always be temporary.

PRs which add `replace` directives in go.mod to change a module to point to a fork will only be
accepted once an upstream PR is opened to officially move the changes to the official module.

Contributors are expected to work with upstream to make their changes acceptable, and remove the
`replace` directive as soon as possible.

If upstream is unresponsive, consider choosing a different dependency or making a hard fork (i.e.,
creating a new Go module with the same source).

[new-issue]: https://github.com/grafana/alloy/issues/new
[code-review-comments]: https://code.google.com/p/go-wiki/wiki/CodeReviewComments
[best-practices]: https://peter.bourgon.org/go-in-production/#formatting-and-style
[uber-style-guide]: https://github.com/uber-go/guide/blob/master/style.md
[CLA]: https://cla-assistant.io/grafana/alloy
[good-first-issue]:
  https://github.com/grafana/alloy/issues?q=is%3Aopen+is%3Aissue+label%3A%22good+first+issue%22
[community-slack]: https://slack.grafana.com/
[example-config]: ../../example-config.alloy
[go-modules]: https://golang.org/cmd/go/#hdr-Modules__module_versions__and_more
