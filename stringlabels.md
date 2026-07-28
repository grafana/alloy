# Pack `discovery.Target` labels into sorted string buffers

Working plan for changing the memory layout of `internal/component/discovery.Target` from two
map-based `commonlabels.LabelSet` fields to a Prometheus-style packed single-string representation
(see `prometheus/model/labels/labels_stringlabels.go`).

## Motivation

`Target` today stores labels as two maps:

```go
type Target struct {
    group commonlabels.LabelSet
    own   commonlabels.LabelSet
    size  int
}
```

Maps are expensive for the access patterns Alloy actually has:

- **Clustering hash** (`HashLabelsWithPredicate`, `target.go:294`) needs labels in deterministic
  order, so it materialises a pooled `[]string` of names, sorts it, then does one `Get` (two map
  lookups) per label. This runs per target on every reshard.
- **Export change detection** (`equality.go:60` → `Target.EqualsTarget`, `target.go:268`) is O(n)
  map lookups per target on every component export.
- **Steady-state memory**: a map with n labels costs ~2n pointers for the GC to scan, per target.
  Alloy routinely holds 10k-100k+ targets live.

A packed, name-sorted string makes hashing a single forward scan (no sort, no pool, no `Get`),
makes equality a pointer plus string compare, and reduces each target to ~2 pointers.

## Decisions

| Decision | Choice |
|---|---|
| Encoding | Hand-rolled, in a new subpackage `internal/component/discovery/internal/labelpack` |
| group/own split | **Survives**: shared `*groupLabels` + per-target packed `own` |
| Empty-valued labels | Normalized away at construction |
| Oversized labels (>= 16 MB) | Panic, matching Prometheus (`labels_stringlabels.go:551`) |
| Rollout | One PR, layout swap only. No exported signature changes, no call-site changes outside `internal/component/discovery` |

### Why hand-roll instead of wrapping `modellabels.Labels`

`prometheus/prometheus v0.312.0`'s `labels_stringlabels.go` is `//go:build !slicelabels &&
!dedupelabels`, so `labels.Labels` *is* already the packed implementation in this build. Wrapping it
would be attractive if `Target` were flat. But because the group/own split survives:

- Two `labels.Labels` cannot be cheaply merged into one view; `data` is unexported, so there is no
  raw-buffer escape hatch.
- `PromLabels()` needs a rebuild from the merged view either way.

So wrapping buys little here, and hand-rolling gives a `Get` that returns `(string, bool)` and a
2-way merge iterator over group+own.

### Why keep the group/own split

`toAlloyTargets` (`discovery.go:225-245`) stores the Prometheus `targetgroup.Group.Labels` map *by
reference* into every target in the group. Kubernetes SD typically has 5-40 shared
`__meta_kubernetes_*` labels across thousands of pods. Flattening would materialise those bytes per
target, and would make `ComponentTargetsToPromTargetGroupsForSingleJob` unable to reconstruct
`targetgroup.Group.Labels` — changing the `Source` strings, which Prometheus' scrape manager keys
target sets by, causing scrape-loop churn on upgrade.

## Encoding

Byte-identical to `prometheus/model/labels` stringlabels: each name and each value is preceded by
its length, encoded as a single byte for length 0..254, or `0xFF` followed by 3 bytes little-endian
for longer. Maximum length `1<<24` (16 MB); panic above that.

Invariants for a packed buffer:

1. Names sorted ascending (byte-wise).
2. Names unique.
3. No empty values.

## Types

```go
// internal/component/discovery/internal/labelpack
type Labels string

func FromLabelSet(ls commonlabels.LabelSet) (Labels, int)
func FromPairs(p []Pair) (Labels, int)   // sorts in place, de-dupes, drops empty values

func (l Labels) Get(name string) (string, bool)             // forward scan, early-exit on sort order
func (l Labels) Range(f func(name, value string) bool) bool
func (l Labels) Iter() Iter                                  // cursor, for merges
func Merge(group, own Labels) MergeIter                      // sorted 2-way merge, own shadows group
```

```go
// internal/component/discovery
type groupLabels struct {
    packed labelpack.Labels
    n      int
    hash   uint64                // cached pure-group StableHash (no-shadowing fast path)
    lsOnce sync.Once
    ls     commonlabels.LabelSet // lazily materialised for targetgroup.Group
}

type Target struct {
    _     [0]func()   // keeps Target non-comparable; MUST be first (trailing zero-size fields get padded)
    group *groupLabels
    own   labelpack.Labels
    size  int32
}
```

`labelpack` lives under `discovery/internal/` so it is importable from the whole
`internal/component/discovery/...` tree but nowhere else, and so its fuzz targets and micro
benchmarks stay isolated from the component tests.

## Files to change

| File | Change |
|---|---|
| `internal/component/discovery/internal/labelpack/*.go` | **New.** Encoder, decoder, `Get`, `Range`, `Iter`, `Merge`, plus tests and fuzz. |
| `internal/component/discovery/target.go:20-355` | Rewrite internals. All exported signatures unchanged. Delete `stringSlicesPool`, `borrowLabelsSlice`, `releaseLabelsSlice` (`:30,38-45`). |
| `internal/component/discovery/target.go:357-408` | `ComponentTargetsToPromTargetGroups*`, see below. |
| `internal/component/discovery/target_builder.go` | Rewrite `targetBuilder` on the `labels.Builder` model (`add []Pair`, `del []string`). |
| `internal/component/discovery/discovery.go:225-245` | `toAlloyTargets`: build `*groupLabels` once per SD group, not per target. |
| `internal/component/discovery/distributed_targets.go` | No change; benefits automatically from a faster `NonMetaLabelsHash`. |

Nothing outside `internal/component/discovery` changes. Verified: no code outside the package
touches `group`/`own`, no `map[discovery.Target]`, no `==` on targets, no reflective JSON/YAML
serialization of `Target`. `NewTargetBuilderFromLabelSets` (exported, `target_builder.go:37`) keeps
its `(group, own commonlabels.LabelSet)` signature.

## Per-method design

- **`ForEachLabel`** (`:123`) → `labelpack.Merge(group.packed, own)`. Sorted order, own shadows
  group, zero allocation. Iteration order changes from map-random to sorted; safe, since the doc
  comment at `:119-122` already disclaims order and every order-sensitive consumer re-sorts.
- **`Get`** (`:156`) → scan `own` (early-exit on sort order), then `group.packed`. O(n) instead of
  O(1), but contiguous and branch-predictable.
- **`HashLabelsWithPredicate`** (`:294`) → biggest win. Single merge scan appending straight into the
  hash buffer. Drops the pooled `[]string`, the `slices.Sort` and the per-label `Get`.
- **`hashLabelsInOrder`** (`:323-355`) → byte emission and the 1 KB `xxhash.Digest` switchover stay
  character-for-character identical. This is the pinned clustering contract (`:320-322`).
- **`groupLabelsHash`** (`:308-318`) → preserve exactly: it hashes group *names* but resolves values
  through `t.Get`, so an `own` override silently changes it. Merge-scan group names with a parallel
  `own` cursor; use the cached `groupLabels.hash` when no shadowing is detected.
- **`EqualsTarget`** (`:268`) → fast path `t.group == o.group && t.own == o.own`.
- **`PromLabels`** (`:97`) → `ScratchBuilder.Add` in merge order, skip the `Sort()` call.
- **`Len`** (`:178`) → still O(1) from `size`.

## `ComponentTargetsToPromTargetGroupsForSingleJob` (`target.go:363-408`)

The structure survives intact because the split survives.

- `Labels:` ← `groupLabels.ls`, materialised once per group under `sync.Once` (parity with today,
  where it is the original map).
- `Targets:` ← **new per-target `LabelSet` decode.** Unavoidable regression with a packed layout.
  Runs once per target per SD update (>= 5 s apart, `discovery.go:166`), and Prometheus'
  `TargetsFromGroup` does more per-target work immediately after.
- `Source` strings (`"%s_part_%v"`, `"%s_rest"`) unchanged, because the group hash is unchanged.
- `labelSetEqualsFn` (`:29`, swapped by `target_test.go:961-964` to fake hash collisions) becomes
  `groupEqualsFn func(a, b *groupLabels) bool`, with pointer identity as the fast path.

## Semantics that must be right

1. **Empty values dropped.** `Get`'s `bool` becomes true presence. Audit the external bool users:
   `pyroscope/java/target.go:15,18,19,24,27,30`, `discovery/process/container.go:45,49,53`,
   `promtailconvert/.../service_discovery.go:30`, `promsdprocessor/consumer/consumer.go:234`. All
   probe `__meta_*` presence, so low risk. `NewTargetFromMap({"a":""})` goes from `Len()==1` to
   `Len()==0`, matching what `targetBuilder` already does (`target_builder_test.go:28-39`).
2. **Empty `own` value shadowing a group label.** `own={a:""}, group={a:"x"}` must still resolve to
   absent, not `"x"`. The tombstone logic currently in `target_builder.go:42-49` moves into
   `NewTargetFromSpecificAndBaseLabelSet`; such a target needs its own `*groupLabels` rather than the
   shared one. Rare, but must be detected at construction, not skipped.
3. **Group interning in `toAlloyTargets`.** `discovery.go:241` passes the same `group.Labels` map for
   every target in the group. Add an unexported constructor taking an already-built `*groupLabels`.
   Getting this wrong turns an O(groups) cost into O(targets).
4. **Builder copy-on-write must survive.** No add/del → return the `group` pointer and `own` string
   verbatim, zero allocation (parity with `target_builder.go:132-134`). Only `own` affected →
   re-encode `own`, share the `group` pointer. A `Del` hitting a group label → a new `*groupLabels`
   per target (the existing TODO at `:174-176`; out of scope, but the packed string is now a natural
   intern key).
5. **`Range` must not revisit labels `Set` during iteration.** Required by
   `LabelMap`/`LabelDrop`/`LabelKeep` (`relabel.go:296-314`), which mutate while ranging. Snapshot
   `add`/`del` into stack arrays, exactly as `labels_common.go:218-231`.
6. **`Target` must stay non-comparable.** `struct{*groupLabels; string; int32}` compiles under `==`,
   but `==` would be semantically wrong (equal content, different group pointers). Today the map
   fields make it a compile error. The `_ [0]func()` field preserves that guard for free.

## Behaviour changes

All of these follow from normalising empty names and values at construction, and none are covered by
an existing test.

1. **A label with an empty value is now absent rather than present-and-empty.** `Get` reports it as
   missing and `Len` does not count it. `TargetBuilder` already behaved this way
   (`target_builder_test.go:28-39`), and Prometheus cannot represent such a label at all, so this only
   affects targets built directly from a map or `LabelSet` that never passed through a builder.
2. **A label with an empty name is dropped.** An empty name is not a valid label name, and `Get("")`
   cannot report it as present without panicking on `name[0]`, so storing it would make it
   unreachable.
3. **Hashes change for targets that carry empty-valued labels.** The old `hashLabelsInOrder` emitted
   `name\xff\xff` for them; they are now absent and contribute nothing. This affects
   `NonMetaLabelsHash`, so during a rolling upgrade the old and new versions would disagree on
   ownership of such targets, causing temporarily duplicated or missed scrapes for those targets
   only. Non-meta labels are rarely empty in practice (`__address__` never is, and anything that went
   through a relabel step already had empty values dropped), but this is the one place where the
   clustering compatibility contract at `target.go:320-322` is knowingly relaxed. Hashes for targets
   without empty-valued labels are byte-identical, as the pinned tests verify.
4. **`groupLabelsHash` changes for groups containing empty-valued labels**, for the same reason,
   which changes the `targetgroup.Group.Source` string and so restarts the affected scrape loops
   once on upgrade.

## Tests

Preserve unmodified (these are the compatibility contracts):

- `target_test.go:591` `TestHashing` — pinned `0xa28155048ff30d6f`, `0xbbbe498586b668f3`
- `target_test.go:797` `TestHashLargeLabelSets` — including `:839` vs `labels.StableHash`
- `target_test.go:852` `TestComponentTargetsToPromTargetGroups` — pinned
  `job_part_9994420383135092995`, `job_part_13313558424202542889`, `job_rest`
- `target_builder_test.go:254-305` — the `builderAdapter` cross-check, incl. `StableHash`
  equivalence at `:270`
- `discovery_test.go:218` `TestToAlloyTargetsOrdersGroupsBySource`
- `distributed_targets_test.go:114,212`

Added:

- `labelpack/labelpack_test.go`, `merge_test.go`: 0/1/many labels; name and value lengths exactly
  254/255/256 (the varint boundary); empty buffer; duplicate and empty-value handling; a panic test
  for the 16 MB limit; `TestEncodingMatchesPrometheus` comparing raw bytes against
  `modellabels.New(...).Bytes(nil)`.
- `labelpack/fuzz_test.go`: `FuzzRoundTrip`, `FuzzEncodingMatchesPrometheus` and `FuzzMerge`. The
  second is the primary safety net for the hand-rolled encoder, since a divergence would silently
  break `StableHash`-based clustering compatibility. ~29M executions, no divergence.
- `target_fuzz_test.go`: `FuzzTargetFromGroupAndOwn`, `FuzzTargetBuilder` and
  `FuzzTargetBuilderRangeMutation`, checking `Target` and `TargetBuilder` against a naive map
  reference and against Prometheus' `labels.Builder` for arbitrary group/own splits. The last one
  covers the mutate-while-ranging contract that `labelmap`/`labeldrop`/`labelkeep` depend on.
- `target_layout_test.go`: guards that `Target` stays non-comparable (`reflect.Type.Comparable`), that
  it stays 32 bytes with the zero-size field first, that the zero value is usable, and that targets
  from one SD group share a single `*groupLabels` pointer before and after relabeling.
- `target_relabel_bench_test.go` (`package discovery_test`, so it can import `common/relabel`
  alongside `discovery`): `Benchmark_Relabel_TargetBuilder` running the real Alloy relabel engine
  over `TargetBuilder` with a realistic Kubernetes rule set. This path was previously unmeasured,
  which is what the comment at `target_test.go:1037` refers to. Plus
  `Benchmark_Relabel_NoChanges`, `Benchmark_Target_EqualsTarget` and `Benchmark_Target_Get`.
- `target_memory_bench_test.go`: `Benchmark_Targets_ResidentMemory`, described above.

## Measured results

Measured on linux/amd64, AMD Ryzen AI MAX+ 395, `-count 6`, compared with `benchstat`.

### Resident memory (`Benchmark_Targets_ResidentMemory`, 100k targets in 20 groups)

This is the headline result and the main justification for the change. It builds targets through
`toAlloyTargets`, drops the service discovery cache, and then samples the heap, so whatever the
targets still reference stays resident. The map layout retained every source `LabelSet` by reference;
the packed layout copies the labels once and lets those maps be collected.

| metric | old (maps) | new (packed) | delta |
|---|---|---|---|
| resident bytes / target | 476 | 251 | **-47%** |
| resident heap objects / target | 6.82 | 1.00 | **-85%** |
| GC ns / cycle with the set live | ~2.5M | ~0.95M | **-62%** |

Note that per-operation `allocs/op` cannot show this, which is why this benchmark exists.

### Throughput

| benchmark | time/op | allocs/op |
|---|---|---|
| `BenchmarkDistributedTargets` (clustering) | **-55.7%** | **-99.2%** (101k -> 1k) |
| `Benchmark_Relabel_NoChanges` | **-61.1%** | **-66.7%** (3 -> 1) |
| `Benchmark_Relabel_TargetBuilder` | **-18.7%** | **-5.2%** |
| `Benchmark_Targets_TypicalPipeline` (net effect) | **-18.5%** | +39.9% |
| `Benchmark_Target_EqualsTarget/unchanged` | **-99.7%** (619ns -> 2ns) | 0 |
| `Benchmark_Target_EqualsTarget/equal_but_rebuilt` | **-97.7%** (617ns -> 14ns) | 0 |
| `Benchmark_Target_EqualsTarget/different` | +23.8% (33ns -> 41ns) | 0 |
| `Benchmark_ToAlloyTargets` | +145.8% | 1 -> 20003 |
| `Benchmark_Target_Get/__address__` (first in `own`) | -7.5% | 0 |
| `Benchmark_Target_Get/job` (in `group`) | +381.8% (9ns -> 44ns) | 0 |

### Regressions, and why they are acceptable

1. **`Benchmark_ToAlloyTargets`: 1 alloc/op -> 20003 alloc/op, +146% time.** The old code stored
   references to the `LabelSet` maps that the Prometheus SD library had already allocated, so
   building a target cost nothing beyond the slice. The new code encodes each target's own labels
   into one buffer, which is one allocation per target and roughly half the bytes of the map it
   replaces. The old benchmark measures only incremental allocation and so flatters the old layout;
   `Benchmark_Targets_ResidentMemory` measures what is actually retained. `toAlloyTargets` runs at
   most every `MaxUpdateFrequency` (5s, `discovery.go:166`).
2. **`Get` on a group label: 9ns -> 44ns.** Inherent to a packed layout: the scan walks `own` and
   then `group` instead of doing two map lookups. Absolute cost is tens of nanoseconds, and `Get` is
   called a handful of times per target per update rather than per sample. Names sort ascending, so
   the scan exits early when it passes the requested name.
3. **`EqualsTarget/different`: 33ns -> 41ns.** The merge walk first compares the packed buffers,
   which for equal group labels is a memcmp before the difference in `own` is found. The cases that
   dominate in practice, unchanged and equal-but-rebuilt, are 44x and 300x faster respectively.
4. **`Benchmark_Targets_TypicalPipeline` allocs +40% while time is -18.5%.** Same cause as (1): the
   encoding cost moved into Alloy from the SD library. Time and resident memory both improve.

### Rejected optimisation

`toAlloyTargets` could pack every target of a group into a single buffer and give each target a
substring of it, reducing allocations from one per target to one per group. Rejected for this PR:
a substring keeps the entire parent buffer alive, so a relabel step that drops most of a group would
retain the dropped targets' bytes indefinitely. Worth revisiting with an explicit re-pack threshold.

## Verification

```sh
# Unit and race tests
go test -race -tags "nodocker gore2regex" ./internal/...

# Encoder fuzzing (the safety net for the hand-rolled encoding)
go test -run '^$' -fuzz FuzzEncodingMatchesPrometheus -fuzztime 5m ./internal/component/discovery/internal/labelpack
go test -run '^$' -fuzz FuzzTargetFromGroupAndOwn    -fuzztime 5m ./internal/component/discovery
go test -run '^$' -fuzz FuzzTargetBuilder            -fuzztime 5m ./internal/component/discovery

# Throughput comparison
go test -run XXX -bench . -benchmem -count 6 -skip ResidentMemory ./internal/component/discovery/ > new.txt
benchstat old.txt new.txt

# Resident memory comparison (absolute metrics, so a single iteration)
go test -run XXX -bench Benchmark_Targets_ResidentMemory -benchtime 1x -count 3 ./internal/component/discovery/

# Lint
golangci-lint run --build-tags "nodocker gore2regex" ./internal/component/discovery/...
go run ./internal/cmd/alloylint ./internal/component/discovery/...
```

Status: all of the above pass. The only failing test in the tree is
`TestPyroscopeJavaIntegration`, which requires root and fails identically on the base commit. The
only lint findings are four pre-existing `goconst` warnings in `discovery/consulagent`,
`discovery/dockerswarm` and `discovery/openstack`, none of which this change touches.

No `go.mod` change, so `make generate-otel-collector-distro` is not required. No `docs/sources`
change; this is internal only. Per `AGENTS.md` the changelog is derived from the PR title, which is
yours to write.

## Out of scope (follow-up PRs)

- Batch-pack a whole SD group into one buffer in `toAlloyTargets`, with an explicit re-pack threshold
  to bound the substring retention described above. This would address the one remaining allocation
  regression.
- Intern duplicate `*groupLabels` produced by relabeling (`target_builder.go:174-176`); the packed
  string is now a natural key.
- Drop `AsMap()` from its two hot callers: `pyroscope/java/loop.go:214` (per profile) and
  `pyroscope/ebpf/ebpf_linux.go:441`.
- Cache `NonReservedLabelSet()` for `loki/source/file/resolver.go:46,77`.
- Add a builder pool or result cache to `discovery.relabel`, which today re-relabels every target
  from scratch on every `Update` with no cache (`relabel/relabel.go:88-105`).
