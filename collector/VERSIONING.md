# How the collector distribution resolves its version (and how it ties to release-please)

> Working notes on the relationship between `release-please`, `CollectorVersion()`,
> and `collector/VERSION`. Written while splitting the "collector reports
> `build.Version`" change into its own PR.

## What release-please is

`release-please` is a release-automation bot that runs in CI. It reads Conventional
Commit messages since the last release, computes the next semantic version, and
opens a **"Release PR."** Two things matter here:

1. The **canonical** current version lives in `.release-please-manifest.json`. That
   is release-please's source of truth.
2. release-please can also keep a version string **in sync across arbitrary files**
   listed in `release-please-config.json`, via the `# x-release-please-version`
   marker. When it bumps the version, its Release PR edits every tracked file.
   Today those are `collector/builder-config.yaml`, `docs/sources/_index.md`, and
   `collector/VERSION`.

Mental model: those tracked files are **outputs** of release-please ("please keep
this number updated for me"), not inputs. The number originates in the manifest.

## The two runtime paths to "the version"

There are two independent ways a version reaches a running binary:

- **Path A — ldflags → `build.Version`.** At build time the Makefile injects
  `-X …build.Version=$(VERSION)` (`Makefile`, `GO_LDFLAGS`), where `$(VERSION)`
  comes from `scripts/image-tag`, which reads `.release-please-manifest.json`. This
  is how `alloy run` and everything using `useragent.Get()` reports its version.
  It is the repo-wide standard.
- **Path B — embed `collector/VERSION` → `CollectorVersion()`.** The collector
  distro's `collector/version.go` does `//go:embed VERSION` and parses the file.
  This is the current behavior on `main`.

## Why Path B (the file) exists — it was *not* always redundant

The file did **not** predate release-please and was **not** made redundant by it —
it was created *because of* release-please.

The file and the embed logic were both introduced on **2026-01-29** in commit
`3ed4a0b3e`, *"Modify collector main.go to resolve version from release please"*.
So it is ~5 months old (since the 1.13.x line), not long-standing. That commit's
purpose was to give the collector-as-its-own-distribution a version source that
release-please bumps and Go embeds — so the collector could report a version "from
release please" via a file.

At that point the file was genuinely **load-bearing**:

- It was the collector binary's runtime version source.
- Because it is `//go:embed`'d, the collector **will not compile** if the file is
  missing.

## Why it becomes redundant *now* (and only now)

The collector binary is built by the **same** Makefile line that injects
`build.Version`:

```make
cd ./collector && $(GO_ENV) go build $(GO_FLAGS) -o ../$(ALLOY_BINARY) .
```

`$(GO_FLAGS)` includes the `build.Version` ldflags. So `build.Version` is *already
present* inside the collector binary, and it already traces back to the same
`.release-please-manifest.json`.

That means **Path A and Path B compute the same number from the same ultimate
source**, just via different plumbing. Changing `CollectorVersion()` to
`return build.Version` simply retires Path B in favor of Path A. Once that lands:

- Nothing embeds or reads `collector/VERSION` anymore.
- The only remaining reference is the `collector/VERSION` entry in
  `release-please-config.json`, which just *bumps* it.
- So release-please would keep editing a file nobody consumes — **that** is the
  redundancy.

Corroboration: even `collector/builder-config.yaml`'s `version:` field (also
release-please-tracked) does not reach the binary, because the code generator
(`collector/generator/generator.go`) *overwrites* the generated version literal
with `CollectorVersion()` at codegen time.

## Direct answers

- **How is it redundant?** It duplicates `build.Version`, which is already in the
  collector binary from the same release-please manifest. The redundancy only
  appears *after* `CollectorVersion()` stops reading the file — it is introduced by
  the `CollectorVersion → build.Version` change, not pre-existing.
- **Can it be removed? Did release-please make it redundant?** release-please
  *created* it, not obsoleted it. It can be removed only *with or after* the embed
  is dropped, and removal must **also** delete the `collector/VERSION` entry from
  `release-please-config.json` — otherwise release-please tries to bump a file that
  no longer exists and can fail the Release PR. Deleting it *today* (while `main`
  still has `//go:embed VERSION`) would break the build.

## Practical sequencing

1. **`CollectorVersion → build.Version` PR** — retires the embed; makes the file
   unread. (Release builds are byte-identical; dev/nightly gain the `-devel+<sha>`
   suffix.)
2. **Optional follow-up cleanup** — delete `collector/VERSION` **and** its
   `release-please-config.json` entry together, in one PR.
