# Major Dependency Updates

## What are major dependencies?

Major dependencies are Go module dependencies of the Alloy project that are known to be complex to upgrade and often resulting in conflicts or breaking changes. At the same time we are committed to update these dependencies regularly in order to receive fixes and improvements from the upstream.

## List of major dependencies

- OpenTelemetry Collector and related dependencies (OTel)
  - The core dependencies have `go.opentelemetry.io/collector/...` import paths.
  - The contrib components have `github.com/open-telemetry/opentelemetry-collector-contrib/...` import paths.
  - NOTE: these typically are all on the same version number for all the Go modules. Same version numbers mean they have been verified upstream to be compatible with each other.
- Prometheus dependencies (Prom)
  - The core and libraries dependencies:
    - `github.com/prometheus/prometheus`
    - `github.com/prometheus/common`
    - `github.com/prometheus/client_golang`
    - `github.com/prometheus/client_model`
- Beyla dependencies (Beyla)
  - `github.com/grafana/beyla/v2`
  - `go.opentelemetry.io/obi`
  - `go.opentelemetry.io/ebpf-profiler`
- Loki dependencies (Loki)

## Major Dependency Relationships

Here is a summary of the relationships between the major dependencies of Alloy.

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
   - Note: Only specific contrib modules depend on Prometheus/Loki, not all of them

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

## Steps to update major dependencies

Don't write anything in this document. Create a separate document called 'deps-update-YYYY-MM-DD.md' in the root of the repository. This is your output file where you will write all the output as required.

### Step 1: Familiarize yourself with the "Tools" section below

Throughout this process, you will be using several tools to gather the information required for accurate decision making. These tools are described in the "Tools" section below. Make sure you familiarize yourself with them before you start.

### Step 2: Establish the latest and current versions of all the major dependencies

For all the major dependencies as listed in "List of major dependencies" section, use the tools described in "Tools" section to find the current and the latest versions.

List these versions in a form of a table containing columns: the dependency name, the current version, the latest version and the emoji indicating whether it needs an update. Write this table to the output. Don't write much more to keep it brief.

Now, the major dependencies also depend on each other. You can see this in the 'Relationships' paragraph above. For each major dependency, take a look at its latest version and see what are the versions of the major dependencies that it uses. For example, we know that Beyla depends on Prometheus client libraries. We want to know what are the versions of these Prometheus client libraries that Beyla pulls in. Create a table for each major dependency that lists the other major dependencies that it pulls. Include columns: dependency name, current version, latest version, an emoji indicating whether the update is required.

The major dependencies that are using the same versions as the ones we want to update to should be denoted with "READY TO GO ✅". Otherwise, recommend what needs to be updated by the owners of the project.

### Step 3: List the current forks and what changes have been added to them

For all the major dependencies as defined above that are replaced with forks, list the changes that have been added to the current fork, using the tools described in the "Tools" section. NOTE: do not investigate forks of Prometheus exporters, as we keep them out of the scope of this process for now.

Make a short summary of the forks: what version they fork from (if it's possible to determine), the list of commits that are added to the fork, and one sentence summary of these changes.

Search for a GitHub issue or upstream PR associated with the fork. They are often mentioned through a comment in the commit message, a PR description on the fork or in the go.mod file. Verify if the required changes were already upstreamed and if we no longer need the fork. Use "Checking if a fork is still needed" tool described below to verify. Always make sure that the changes required are indeed part of the new version and are already released. Otherwise, we may need to keep the fork.

If the fork is using a branch or a tag with certain naming convention that can be continued, determine the expected name of the new branch or tag in the fork that can use the latest version of the upstream major dependency as the base.

Determine what is the status of the fork:

- If the fork is no longer needed, quote the issues or PRs that resolve it. Denote with ✅
- If a new, updated tag or branch exists, write clearly that an updated fork of the new version exists and we can continue. Denote with ✅
- In the case of `go.opentelemetry.io/obi => github.com/grafana/opentelemetry-ebpf-instrumentation` and `go.opentelemetry.io/ebpf-profiler => github.com/grafana/opentelemetry-ebpf-profiler` replaces, we want to pick the latest version from the grafana fork as it is the most up to date. Determine that version and denote with ✅
- If it doesn't exist, write clearly that we need to update the fork before we can continue. State what upstream version should be used as the base. Denote with 🛑

Only continue to the next step if all the major dependenies have a fork ready, don't need one, or you're told to continue. If there are forks that are not ready but not for the major dependencies, we can continue and keep them unchanged.

### Step 4: Update Go modules to desired versions

Having determined the desired versions of the major dependencies, update the go.mod file to use the desired versions. Make sure you keep in mind the relationships between the major dependencies as described in the "Major Dependency Relationships" section above.

You know the update is successful if `go mod tidy` can successfully resolve the dependencies.

If you encounter conflicts, you need to resolve them. You can do this by:

- Trying to use an earlier version of one of the dependencies that are involved in the conflict. You can inspect the code or changelog to determine which one is best to downgrade.
  
- If there is an existing replace directive for the problematic dependencies, try to remove it and see if `go mod tidy` can successfully pass - perhaps it is no longer needed. If that's the case, call it out in summary. It may be a fork that we no longer need.
  
- If you are unable to resolve the conflicts, call it out and recommend the next steps. Make sure you clearly and briefly classify the kind of issue:
  - Is it that a major dependency upstream is lagging behind with updating some other dependency?
  - Is it that some dependency has breaking changes?
  - What would be the best approach to handle the issue? Can we avoid forking?
  - What would be the simplest and fastest approach to handle the issue? Can we update to an older version of major dependencies instead while the conflicts are resolved?

- DO NOT simply declare that 'this is an upstream issue'. In order to do that, explore the changelog and isolate specific commits and changes that happened upstream with specific examples of why this is an upstream issue.

- Under no circumstance should you vendor or create some kind of 'local fork' of a dependency. Instead try harder to find the solution.

If you were able to produce a working go.mod file and the `go mod tidy` command passes, don't yet worry about building the project or running the tests. These are the next steps to be taken later.

Proceed to the next step only if `go mod tidy` is successful or you are asked to do so.

### Step 5: Organize go.mod

Make sure you organise the go.mod in the following way:

- module name, go version, etc.
- direct dependencies in one require() block
- indirect dependencies in another require() block
- all the replace directives in separate lines with comments
- anything else

After reorganising the go.mod, make sure you run `go mod tidy` again to make sure it is still successful and properly formatted.

### Step 6: Fix compilation errors

If you run `go build` or `go test` directly, you may not get the correct build tags. Make sure you use them. These can be found in the Makefile. You can also use `make alloy` to build the whole project.

Start fixing the compilation errors in the following order, which ensures we start with more fundamental issues and work our way down to the more complex ones:

- `./internal/runtime/...` - to fix parts of the core Alloy runtime
- `./internal/component/prometheus/...` - to fix the Prometheus components
- `./internal/component/loki/...` - to fix the Loki components
- `./internal/component/otelcol/...` - to fix the OpenTelemetry Collector components
- `./internal/component/beyla/...` - to fix the Beyla components
- `./internal/component/pyroscope/...` - to fix the Pyroscope eBPF components
- `./internal/component/...` - to fix all components
- `./internal/converter/...` - to fix the config converters
- `make alloy` - to build the whole project and make sure it compiles

As you encounter errors, you need to fix them. Use the tools described in the "Tools" section below to help you. Make sure you try the following approaches:

- Isolate the dependencies and packages and errors that are involved, so you can focus on solving one issue at a time. Establish what was the previous version of the dependency and what is the new version.

- Fetch the commit history between these versions and look for information in the commit messages and PR descriptions and associated issues to understand what has changed.

- Look at the code changes between the versions to understand how the code has changed that led to compilation errors.

- If the issue is with the Alloy code, we will often be able to update the code to fix the issue. Do that if possible.

- If the issue is in another Alloy dependency, maybe we can:
  - See if the upstream has a more recent commit on main/master branch (use `gh` command to find that out) that we can use.
  - See if we can downgrade one or the other dependency that are in conflict.
  - See if we can upgrade one or more dependencies.
  - Try to come up with some other ways to solve it if possible.
  - DO NOT GIVE UP! We really need to fix these issues in Alloy code rather than creating forks or blocking on upstream changes.

- DO NOT simply declare that 'this is an upstream issue'. In order to truly establish that, explore the changelog and isolate specific commits and diff the changes that happened upstream to provide specific examples of why this is an upstream issue that cannot be solved by changing dependency versions or the Alloy code.

- Under no circumstance should you vendor or create some kind of 'local fork' of a dependency. Instead try harder to find the solution.

If you were able to fix the compilation errors, explain what you did and how confident you are that these are correct fixes. You can now proceed to the next step.

If we still have compilation errors, stop here and explain briefly what issues are left to resolve. Provide snippets of `go build` commnads that we can run to reproduce the errors and continue the investigation. Provide the options we have and your recommendations. Be concise and clear.

### Step 7: Fix test errors and failures

This is a very important step, we're almost there, but we need to make sure all the tests pass. Do not give up or leave
a TODO item without exploring multiple alternatives, some of which are called out in this document in the
current and previous steps. If you do find an issue that is too hard for you to solve, stop and describe
it clearly as required, provide steps to reproduce it, the options we have and your recommendations.

NOTE: If you run `go build` or `go test` directly, you may not get the correct build tags. Make sure you use them. These can be found in the Makefile.

Start fixing the test errors and failures in the following order, which ensures we start with more fundamental issues and work our way down to the more complex ones:

- `./internal/runtime/...` - to fix parts of the core Alloy runtime
- `./internal/component/prometheus/...` - to fix the Prometheus components
- `./internal/component/loki/...` - to fix the Loki components
- `./internal/component/otelcol/...` - to fix the OpenTelemetry Collector components
- `./internal/component/beyla/...` - to fix the Beyla components
- `./internal/component/pyroscope/...` - to fix the Pyroscope eBPF components
- `./internal/component/...` - to fix all components
- `./internal/converter/...` - to fix the config converters
- `make test` - to run all the tests and make sure they all pass

As you encounter errors, you need to fix them. Use the tools described in the "Tools" section below to help you. Make sure you try the following approaches:

- Isolate the dependencies and packages and errors that are involved, so you can focus on solving one issue at a time. Establish what was the previous version of the dependency and what is the new version.

- Fetch the commit history between these versions and look for information in the commit messages and PR descriptions and associated issues to understand what has changed.

- Look at the code changes between the versions to understand how the code has changed that led to test errors and failures.

- Test changes may possibly indicate a significant change in the behaviour and may need to be documented as a new feature, breaking change, bugfix or exposed to the users and documented. Help me identify such situations and give a recommendation. Be brief and to the point.

- When trying to fix a failing test, do consider what are the dependencies that we are using, are there any suspicious replace directives and are the versions mismatching for some reason? Don't hesitate to go back to figuring out what are the right versions of dependencies that can be used to address the issue at hand.

- Don't be too quick to conclude that there is an upstream bug. It's relatively rare and it is much more frequent that we are using mismatched versions of these dependencies or that we are doing something wrong in our code. Investigate all test failures thoroughly before concluding they're upstream bugs. Try to find workarounds or fixes in our codebase first, and don't assume upstream bugs without exhausting all options.  

- If you think you found a real issue upstream, search the issues and PRs, maybe there is someone who has already found it and maybe there are fixes already in the main. Use the Tools descirbed in this doc. If the issue is fixed upstream, you can switch to use that commit SHA after merge (important! take the commit SHA that is on the main branch upstream, not development branch).

- Under no circumstance should you vendor or create some kind of 'local fork' of a dependency. Instead try harder to find the solution.

Consider this step successful if all the tests pass and there were no big changes to test expectations.

If the tests pass, but we had to change the test expectations in a meaningful way that will impact end users, explain what is the breaking change.

If after all your best efforts there are remaining test failures, make sure you give me a snippet command on how to run that specific test so I can quickly run it on my machine and see what is going on. Provide description of what you think is failing and how does it relate to our dependency updates.

### Tools

#### Finding latest releases on GitHub (preferred method)

```bash
gh release list -R prometheus/prometheus -L 20
```

Skip RC releases or patches to previous LTS releases. Look for the latest stable release by semantic versioning.

#### Finding latest releases on Go package manager (alternative method)

Use this command to find the latest releases on the Go package manager:

```bash
go list -m -versions <package> | tr ' ' '\n'
```

Be careful to filter out any versions that are not proper semantic versioning releases. Then you typically want to pick the lastest version as ordered by the semantic versioning convention.

#### Figuring out latest `github.com/prometheus/prometheus` dependency version

If you find on GitHub a release of prometheus/prometheus, for example `v3.4.2`, you need to translate it into a Go module version: The Go module version starts with a `v0.` followed by the major version, and the minor version expressed as two digits (so would have a leading zero if needed). Then comes the `.` followed by the patch version.

So in our example, the `v3.4.2` release would be translated into the `v0.304.2` Go module version.

You may need to do the reverse of this conversion to resolve a GitHub tag from a Go module version.

Also, similar convention may apply to Loki dependency.

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

#### Getting list of changes that have been added to a forked dependency

List changes in fork `grafana/prometheus` branch `staleness_disabling_v3.7.3` compared to upstream `main`:

```bash
gh api repos/prometheus/prometheus/compare/main...grafana:staleness_disabling_v3.7.3 --jq '.commits[] | "\(.sha[0:7])  \(.commit.author.date)  \(.commit.author.name)  \(.commit.message|split("\n")[0])"'
```

#### Getting list of changes made to a dependency between two versions

List changes in `prometheus/prometheus` between `v3.7.1` and `v3.7.3`:

```bash
gh api repos/prometheus/prometheus/compare/v3.7.1...v3.7.3 --jq '.commits[] | "\(.sha[0:7])  \(.commit.author.date)  \(.commit.author.name)  \(.commit.message|split("\n")[0])"'
```

For each change, use "Getting PR details" to fetch full PR descriptions.

#### Getting PR details

Extract PR number from commit SHA:

```bash
gh api repos/prometheus/prometheus/commits/1195563 --jq '.commit.message | scan("#[0-9]+")'
```

Fetch PR description by number:

```bash
gh pr view 17355 -R prometheus/prometheus --json title,body,url
```

#### Getting changelog from GitHub release notes

Fetch release notes and changelog:

```bash
gh release view v3.7.3 -R prometheus/prometheus --json tagName,name,publishedAt,body
```

#### Checking if the issue is already known upstream

Search for issues mentioning an error:

```bash
gh issue list -R open-telemetry/opentelemetry-collector-contrib -S "loadbalancer does not have type otlp" -L 10
```

Add filters: `is:open`, `is:closed`, `label:bug`, `author:username` to narrow results.

#### Viewing issue details

View full issue details including status, description, and comments:

```bash
gh issue view 44054 -R open-telemetry/opentelemetry-collector-contrib --json number,title,state,body,url,createdAt,closedAt
```

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

#### Checking if a fork is still needed

To determine if a fork can be removed, follow this investigation pattern:

**1. Understand what the fork changes and check for upstream references:**

```bash
# Get fork commit message (may reference upstream issues/PRs)
gh api repos/grafana/repo-name/commits/branch-or-sha --jq '.commit.message'

# Get fork PR description if the commit was part of a PR
gh api repos/grafana/repo-name/commits/branch-or-sha/pulls --jq '.[0] | .number, .title, .body'

# Extract issue/PR numbers if present (look for #1234 or full URLs)
# Then check their status:
gh issue view 1234 -R upstream-org/repo-name --json state,title,closedAt
gh pr view 5678 -R upstream-org/repo-name --json state,title,mergedAt
```

**2. Compare the files the fork modified with latest upstream:**

```bash
# Get list of changed files in the fork
gh api repos/grafana/repo-name/commits/branch-or-sha --jq '.files[] | .filename'

# Compare those same files between old and new upstream versions
module=github.com/upstream-org/repo-name
diff -u \
  "$(go env GOMODCACHE)/${module}@old_version/path/to/file.go" \
  "$(go env GOMODCACHE)/${module}@new_version/path/to/file.go"
```

**3. Search for related upstream activity if no direct references:**

```bash
# Search recent commits in the same paths the fork modified
gh api 'repos/upstream-org/repo-name/commits?path=relevant/path&sha=main' --jq '.[0:15] | .[] | "\(.sha[0:7])  \(.commit.author.date)  \(.commit.message|split("\n")[0])"'

# Search PRs with keywords from fork purpose
gh pr list -R upstream-org/repo-name -S "keywords from fork" --state merged -L 10
```
