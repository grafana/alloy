# Major Dependency Updates

## What are 'major dependencies?

These are dependencies that are known to be complex to upgrade and often resulting in conflicts or breaking changes.
The following dependencies of Alloy are currently considered to be 'major' dependencies:

- OpenTelemetry Collector dependencies (OTel)
  - Especially the core repository dependencies that can be recognized by the `go.opentelemetry.io/collector/...` import paths
  - Secondly the components coming from the `opentelemetry-collector-contrib` repository that can be recognized by the `github.com/open-telemetry/opentelemetry-collector-contrib/...` import paths
  - Note that OTel dependencies are split into multiple go modules, but they are usually using the same version number
    for all the modules within the same repository.
- Prometheus dependencies (Prom)
  - Especially the core repository dependencies such as:
    - `github.com/prometheus/prometheus`
    - `github.com/prometheus/common`
    - `github.com/prometheus/client_golang`
    - `github.com/prometheus/client_model`
  - Secondly the remaining dependencies such as exporters.
  - Thse are typically using version numbers that are independent from other Prometheus dependencies.
- Beyla dependencies (Beyla)
  - This is not just the `github.com/grafana/beyla/v2` but also `go.opentelemetry.io/obi` which has been donated to the OpenTelemetry project.
- Loki dependencies (Loki)
  - These can be recognized by the `github.com/grafana/loki/...` import paths.
  - The version numbers often follow the same pattern as the Prometheus dependencies.

Getting onto the new versions of OTel, Prometheus and Beyla are usually quite important for us, as there are often a lot of fixes and improvements.

## Major Dependency Relationships and update order

Here is a summary of the relationships between the major dependencies of Alloy, which also determines
the recommended update order of the major dependencies.

1. **Prometheus Client Libraries** (client_golang, client_model, common)
   - These are foundational libraries with no dependencies on other major dependencies
   - Used by: Prometheus, Beyla, Loki, and Alloy directly

2. **OpenTelemetry Collector Core**
   - Does not depend on Prometheus, Beyla, or Loki
   - Used by: Prometheus, Beyla, Loki, contrib packages, and Alloy directly

3. **OpenTelemetry Collector Contrib**
   - Depends on: OpenTelemetry Collector Core
   - Depends on: Prometheus (via `exporter/prometheusexporter`, `exporter/datadogexporter`, `pkg/translator/loki`)
   - Depends on: Loki (via `pkg/translator/loki`)
   - Does NOT depend on: Beyla
   - Note: Only specific contrib packages depend on Prometheus/Loki, not all of them

4. **Prometheus** (prometheus/prometheus)
   - Depends on: OpenTelemetry Collector Core (component, pdata, processor packages)
   - Depends on: OpenTelemetry Collector Contrib (processor/deltatocumulativeprocessor, internal/exp/metrics, pkg/pdatautil)
   - Depends on: Prometheus client libraries (client_golang, client_model, common)
   - Does NOT depend on: Beyla or Loki

5. **Beyla** (grafana/beyla/v2)
   - Depends on: Prometheus client libraries (client_golang, client_model, common)
   - Depends on: OpenTelemetry Collector Core (component, pdata, exporter packages)
   - Depends on: OBI (go.opentelemetry.io/obi)
   - Does NOT depend on: Prometheus (full) or Loki

6. **Loki** (grafana/loki/v3)
   - Depends on: Prometheus (full prometheus package, not just client libs)
   - Depends on: Prometheus client libraries (client_golang, client_model, common)
   - Depends on: OpenTelemetry Collector Core (pdata, component packages)
   - Depends on: OpenTelemetry Collector Contrib (internal/exp/metrics, pkg/pdatautil, processor/deltatocumulativeprocessor)
   - Does NOT depend on: Beyla

## Steps to update a major dependency

Don't write anything in this document. Create a separate document called 'deps-update-YYYY-MM-DD.md' in the root of the repository. For each step write a short summary of what you did and how successful you were + any other output that is
specifically required for that step.

### Step 1: Establish the latest and current versions of all the major dependencies

For all the major dependencies, use the tools described in this document to find the latest versions.
List them in a form of a table containing three columns: the dependency name, the current version and the latest version.

### Step 2: List the current forks and what changes have been added to them

For all the dependencies that are replaced with forks, list the changes that have been added to the current fork, using the tools described in this document.

Make a short summary of the forks and recommend whether the change can be upstreamed or if we need to continue maintaining the fork.

### Step 3: Update the major dependencies in the recommended order

Update the major dependencies in the recommended order, using the tools described above in this document:

- Initially keep the forks unchanged.
- For each major dependency, perform these steps:
  - Prefer targeting published tags explicitly: `go get <module>@vX.Y.Z`. Avoid using `go get -u` or leaving pseudo-versions unless you have documented why the tag is unavailable.
  - Update the version in the go.mod file to the latest version
  - Check if `go mod tidy` can successfully resolve the dependencies. If it can, move on to the next dependency.
  - If you encounter issues with the forks, call it out and recommend
    the next steps, which may require creating a new fork, based on the latest version of the dependency.
  - If you encounter conflicts:
    - Try to use an earlier version of the dependencies that are involved in the conflict.
      This may require adding replace directives to the go.mod file. This may also require tracing your steps back
      to the earlier dependencies that were updated.
    - If there is an existing replace directive for the problematic dependencies, try to remove it and see if `go mod tidy` can successfully pass - perhaps it is no longer needed. If that's the case, call it out in summary.
  - If you are unable to resolve the conflicts, call it out and recommend the next steps.
    Make sure you clearly classify the kind of issue:
    - Is it that a major dependency upstream is lagging behind with updating some other dependency?
    - Is it that some dependency has breaking changes?
    - What would be the best approach to handle the issue?
    - What would be the simplest and fastest approach to handle the issue?

- If you were unable to update all dependencies in a way that `go mod tidy` can successfully pass, summarise how
  far you were able to get and what is still left to resolve.

- If you were able to produce a working go.mod file and the `go mod tidy` command passes, don't yet worry about
  building the project or running the tests. These are the next steps to be taken later.

- Organise the go.mod in the following way:
  - module name, go version, etc.
  - direct dependencies in one require() block
  - indirect dependencies in another require() block
  - all the replace directives in separate lines with comments
  - anything else

### Step 4: Fix `make alloy` compilation errors

- Use `make alloy` as it correctly sets the build tags. If you run `go build` or `go test` directly, you may not get the correct build tags. These can be found in the Makefile.
- If you get compilation errors:
  - List the errors and the dependencies that are involved in the errors. State what version we used previously and what version we updated to.
  - With that information you can fetch the commit history using similar method we used to check what were the changes in the forks. You can get even more details by inferring the pull request URL and reading the PR description and comments.
  - With information about changes history, can you point to what are the likely issues and how they can be solved?
  - Check if the compilation error can be solved relatively easy:
    - If the issue is with the Alloy code, we should be able to update the code.
    - If the issue is in other Alloy dependency, maybe we can:
      - See if the upstream has a more recent commit on main/master branch (use GH api to find that out) that we can use. - See if we can downgrade one or the other dependency that are in conflict.
      - See if there are some other ways to solve it.
      - If there is no simple solution, report this problem clearly and you can stop here. We want to work on one problem at a time.
- If you were able to fix the compilation errors, explain what you did and how confident you are that these are correct fixes.
- If we still have compilation errors, stop here.

### Step 5: Fix test errors and failures

This is a very important step, we're almost there, but we need to make sure all the tests pass. Do not give up or leave
a TODO item without exploring multiple alternatives, some of which are called out in this document in the
current and previous steps. If you do find an issue that is too hard for you to solve, stop and describe
it clearly as required, provide steps to reproduce it, the options we have and your recommendations.

- If you run `go build` or `go test` directly, you may not get the correct build tags. Make sure you use them.
  These can be found in the Makefile.

- This step is basically the same as the previous step where we fixed the `make alloy` compilation errors,
  but for the tests. Follow the same principles as in the previous step to diagnose and fix issues, but instead of using `make alloy`, use the following commands to progressively fix more complex test packages and test suites:
  - `go test ./internal/runtime/...` - to test parts of the core Alloy runtime
  - `go test ./internal/component/prometheus/...` - to test the Prometheus components
  - `go test ./internal/component/loki/...` - to test the Loki components
  - `go test ./internal/component/otelcol/...` - to test the OpenTelemetry Collector components
  - `go test ./internal/component/beyla/...` - to test the Beyla components
  - `go test ./internal/component/pyroscope/...` - to test the Pyroscope eBPF components
  - `go test ./internal/component/...` - to test all components
  - `go test ./internal/converter/...` - to test the config converters - make sure that users can still smoothly convert
    their configurations to Alloy. If there is a lot of additions to alloy output files, this often indicates that
    the defaults are not correctly handled in component configs.
  - And finally use `make test` to run all the tests and make sure they all pass. DO NOT SKIP THIS STEP.

- If you do find that you need to modify Alloy tests, make a clear note and explain why this has to happen:
  - Differentiate between tests failing to compile vs. tests failing to pass. The latter is often more concerning.
  - Test changes may possibly indicate a significant change in the behaviour and may need to be documented as
    a new feature, breaking change, bugfix or exposed to the users and documented. Help me identify such situations
    and give a recommendation. Be brief and to the point.

- If a test fails because the behavior after the update is different from the behavior before
  the update, review the changes in the relevant upstream dependencies between the previous and
  new versions (see the tips and tools section below for methods to do this). There may be a
  clear explanation for why the test fails, which can help us fix the issue or identify it as
  a breaking change.

- Don't be too quick to conclude that there is an upstream bug. It's relatively rare and it is
  much more frequent that we are using mismatched versions of these dependencies or that we are
  doing something wrong in our code. Investigate all test failures thoroughly before concluding
  they're upstream bugs. Try to find workarounds or fixes in our codebase first, and don't assume
  upstream bugs without exhausting all options. Check if similar patterns in the codebase handle
  the same issue.

- If you think you found a real issue upstream, search the issues and PRs, maybe there is someone
  who has already found it and maybe there are fixes already in the main. See the paragraph below
  about checking if the issue is already known upstream. If it is fixed upstream, you can switch to
  use that commit SHA after merge (important! take the commit SHA that is on the main branch upstream)
  by following the steps in a section about using a specific commit for a go.mod dependency below.

- When trying to fix a failing test, do consider what are the dependencies that we are using, are there any suspicious
  replace directives and are the versions mismatching for some reason? Don't hesitate to go back to figuring out what
  are the right versions of dependencies that can be used to address the issue at hand.

- If after all your best efforts there are remaining test failures, make sure you give me a snippet command on how to run that specific test so I can quickly run it on my machine and see what is going on.

### Tips and tools to use

#### Finding latest releases on GitHub (preferred method)

Use this command to find the latest releases on GitHub:

```bash
curl -s "https://api.github.com/repos/<owner>/<repo>/releases?per_page=20" | jq '.[].tag_name'
```

Make sure you skip the RC releases or anything that doesn't look like a proper release. Take care as some releases might
be patches to a previous LTS release. We typically are looking for the lastest stable release as ordered by the semantic versioning convention.

#### Finding latest releases on Go package manager (alternative method)

Use this command to find the latest releases on the Go package manager:

```bash
go list -m -versions <package> | tr ' ' '\n'
```

Be careful to filter out any versions that are not proper semantic versioning releases. Then you typically want to pick the lastest version as ordered by the semantic versioning convention.

#### Figuring out latest `github.com/prometheus/prometheus` dependency version

If you find on GitHub a release of prometheus/prometheus, for example `v3.4.2`, you need to translate it
into a Go module version: The Go module version starts with a `v0.` followed by the major version, and a the minor version expressed as two digits (so would have a leading zero if needed). Then comes the `.` followed by the patch version.

So in our example, the `v3.4.2` release would be translated into the `v0.304.2` Go module version.

#### Viewing dependencies of a Go module

```bash
go mod download <module>@<version>
cd $(go env GOMODCACHE)/<module>@<version>
go mod graph # to view the dependency graph
cat go.mod # to view the go.mod file for details of what is direct / indirect dependencies
```

For example, to view the dependencies of prometheus/prometheus v0.304.2, you would run:

```bash
go mod download github.com/prometheus/prometheus@v0.304.2 && cd $(go env GOMODCACHE)/github.com/prometheus/prometheus@v0.304.2 && cat go.mod
```

#### Getting list of changes that has been added to a forked dependency

For example, if we see that there is a replace directive for `prometheus/prometheus` in the go.mod file with
a fork named `github.com/grafana/prometheus` using `staleness_disabling_v3.7.3` branch, we can get the list of changes that has been added to the fork by running:

```bash
curl -s -H "Accept: application/vnd.github+json" \
  https://api.github.com/repos/prometheus/prometheus/compare/main...grafana:staleness_disabling_v3.7.3 \
| jq -r '.commits[] | "\(.sha[0:7])  \(.commit.author.date)  \(.commit.author.name)  \(.commit.message|split("\n")[0])"'
```

#### Getting list of changes made to a dependency between two versions

For example, if we want to get the list of changes made to `prometheus/prometheus` between versions `v0.305.1` and `v0.307.3`, we can run:

```bash
curl -s -H "Accept: application/vnd.github+json" \
  https://api.github.com/repos/prometheus/prometheus/compare/v0.305.1...v0.307.3 \
| jq -r '.commits[] | "\(.sha[0:7])  \(.commit.author.date)  \(.commit.author.name)  \(.commit.message|split("\n")[0])"'
```

You can go fetch the related PRs to read the descriptions and comments to understand the changes further.
You can also look for the changelog file or notes attached to the release page on GitHub.

#### Checking if the issue is already known upstream and if there is a fix available

It may be the case that the issue you found has already been reported upstream and there could be a fix available.

Do check for this by searching the issues and PRs on GitHub. For example, if we want to search for issues in open-telemetry/opentelemetry-collector-contrib that mention the error message "loadbalancer does not have type otlp", we can run:

```bash
owner=open-telemetry
repo=opentelemetry-collector-contrib
error_message='loadbalancer does not have type otlp'
per_page=10

# Build and URL-encode the GitHub search query

q="repo:$owner/$repo is:issue $error_message"
encoded_q=$(printf '%s' "$q" | jq -sRr @uri)

curl -s \
  -H "Accept: application/vnd.github+json" \
  -H "X-GitHub-Api-Version: 2022-11-28" \
  "<https://api.github.com/search/issues?q=$encoded_q&per_page=$per_page>" \
| jq -r '.items[] | "\(.number)\t\(.state)\t\(.created_at)\t\(.title)\n\(.html_url)\n"'

```

You can add qualifiers like in:title, in:body, label:bug, author:username, or a time window like created:>=2025-01-01 to narrow results.
You can also switch to open issues by setting state=open

#### Using a specific commit for a go.mod dependency

Suppose you want to use `9cc36524215aaa92192ac3faf5c316a6b563818a` commit for `github.com/open-telemetry/opentelemetry-collector-contrib/exporter/loadbalancingexporter`. It's hard to figure out the version name to use, so follow the following steps:

Add a temporary replace:

```go.mod
replace github.com/open-telemetry/opentelemetry-collector-contrib/exporter/loadbalancingexporter => github.com/open-telemetry/opentelemetry-collector-contrib/exporter/loadbalancingexporter 9cc36524215aaa92192ac3faf5c316a6b563818a
```

Run `go mod tidy` and it will fix the raw commit sha with the correct version number corresponding to the commit you want!

#### Inspecting upstream code changes between versions

When dependencies introduce new features or breaking changes, you can inspect the changes directly without cloning repositories:

```bash
old=v0.138.0
new=v0.139.0
module=github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kafkaexporter
go mod download ${module}@${old} ${module}@${new}

# Inspect specific files (e.g., config structs, factory code, interfaces)
diff -u \
  "$(go env GOMODCACHE)/${module}@${old}/config.go" \
  "$(go env GOMODCACHE)/${module}@${new}/config.go"

# Or compare entire directories to spot new files or removed code
diff -ur \
  "$(go env GOMODCACHE)/${module}@${old}" \
  "$(go env GOMODCACHE)/${module}@${new}" | head -200
```
