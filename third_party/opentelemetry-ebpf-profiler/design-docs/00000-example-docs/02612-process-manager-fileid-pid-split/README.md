> [!IMPORTANT]
>
> This document is from Elastic times and precedes the open-sourcing of the
> profiling agent. It is provided as an example of what a design document
> could look like. Many of the links had to be removed because the original
> git history of the profiling agent had to be erased for legal reasons when
> it was open-sourced.

Process Manager: Move Per-FileID Info Into Dedicated Manager
============================================================

## Meta

- **Author(s)**: Joel Höner
- **Start Date**: Jul 13, 2022
- **Goal End Date**: Jul 29, 2022
- **Primary Reviewers**: Christos Kalkanis, Florian Lehner

## Problem

The process manager in the host agent mixes up multiple related but still
distinct concerns in a single type. It manages both information that is kept per
process ID (PID) as well as information kept per file ID. The entanglement makes
consistency expectations between the different information being stored harder
to fathom. The per-file-ID information being stored additionally has
underdocumented and inconsistent lifetimes, with part of the information being
stored in append-only maps (inserted and kept forever), while other information
is reference-counted. The reference counting for the latter is mixed in with the
other business logic of the process manager without any form of abstraction.

## Success Criteria

- Clear separation of per-PID and per-file-ID information in two separate
  managers
- Clear and consistent lifetime for all per-file-ID information being managed
- Clearly communicate mutability/constness of the data being stored
- New manager must not call back into the PM that owns it
    - We have too many circle-references already

## Scope

- This document is concerned only with first splitting the per-file-ID
  information storage and management out from the process manager and improving
  on how it is managed
- Improvements on any of the per-PID information within the process manager are
  not in scope

## Context

In the following sections we'll start by taking inventory of the per-file-ID
information that is currently being stored in the process manager, its use,
lifetime and mutability.

### The `interpreterData` map

The [`interpreterData` map][interpreter-data-field] serves as a cache of
per-executable interpreter data ([`interpreter.Data`][interpreter-data-struct].

[interpreter-data-field]: #
[interpreter-data-struct]: #

When a new mapping is reported, it is used to quickly determine whether we have
already previously concluded that the `FileID` associated with the new mapping is
definitely not an interpreter [by checking for a `nil` record in
`interpreterData`][interpreter-data-check]. If this is not the case (either no
entry exists or the entry is non-`nil`), the [`pm.interpreterLoader` array] is
[walked][array-walking], asking each interpreter unwinder to attempt to detect
whether they want to handle the file. If an interpreter unwinder detects the
file as being supported by them, we store the corresponding helper information
that the interpreter extracted in the `interpreterData` map. If the search for a
suitable interpreter unwinder is exhausted unsuccessfully, we instead store
`nil`.

[interpreter-data-check]: #
[`pm.interpreterLoader` array]: #
[array-walking]: #

The map is inserted into in `attachInterpreter` via the following
call-chains:

- [`ProcNewMapping`](#) → [`handleNewInterpreter`](#) → [`attachInterpreter`](#)
  - [`ProcNewMapping`](#) → [`handleNewInterpreter`](#) → go [`handleNewInterpreterRetry`](#) → [`attachInterpreter`](#)

The map is read in:

- [`attachInterpreter`](#)
- [`handleNewInterpreter`](#)

Records are never deleted from this map, nor are records that were inserted ever
mutated.

### The `fileIDInfo` map

The [`fileIDInfo` map] is harder to reason about. Its main purpose is to store
the information required to delete all information for a given FileID from the
two BPF maps `stack_delta_page_to_info` and `exe_id_to_%d_stack_deltas`.

[`fileIDInfo` map]: #

It further stores information about gaps in the unwinding information of
executables which is passed to [`interpreter.New`]. The information is currently
only consumed by the V8 unwinder that uses it to find the pre-compiled JIT block
within the binary.

[`interpreter.New`]: #

Entries in the map are managed via reference counting: incrementing the RC is
performed mixed into the business logic of the process manager ([link
1][rc-inc-1], [link 2][rc-inc-2]) while decrementing the RC is performed in the
[`decreaseReferenceCount`][rc-dec] function.

[rc-inc-1]: #
[rc-inc-2]: #
[rc-dec]: #

Entries are inserted into the `fileIDInfo` map in either
[`ProcNewMapping`][file-id-info-insert-1] or
[`AddSynthIntervalData`][file-id-info-insert-2]. In `ProcNewMapping`, if a
record for a file ID already existed previously, we instead just increase the
RC. `AddSynthIntervalData`, on the other hand, errors out when passed a file ID
that is already known. [`ProcNewMapping`][file-id-info-insert-1] is the typical
code-path taken in the majority of executables whereas
[`AddSynthIntervalData`][file-id-info-insert-2] is more of an edge-case; it
provides the ability to override unwinding information for executable with
broken `.eh_frame` sections. It's currently only used for `[vdso]` handling on
aarch64.

[file-id-info-insert-1]: #
[file-id-info-insert-2]: #

The value to be stored in the map -- for both code paths -- is returned from the
[`loadDeltas`][load-deltas-return] function. [`loadDeltas`][load-deltas-return]
loads unwinding information into the `stack_delta_page_to_info` and
`exe_id_to_%d_stack_deltas` BPF maps and then returns a newly created instance
of the [`fileIDInfo` struct][file-id-info-struct].

[load-deltas-return]: #
[file-id-info-struct]: #

The reference counter can be decreased via the following code-paths:

- [`ProcExit`](#) → [`deletePIDAddress`](#) → [`decreaseReferenceCounter`](#)
- [`ProcMunmap`](#) → [`deletePIDAddress`](#) → [`decreaseReferenceCounter`](#)

In `deletePIDAddress`, once the RC reaches 0, the information from the
map is used to remove entries corresponding to the FileID from the BPF
maps ([link 1](#), [link 2](#)) and then also the entry itself from the
`fileIDInfo` map.

The map is read using the following code-paths:

- [`New`](#) → [`collectInterpreterMetrics`](#) → anonymous spawned goroutine
- [`ProcExit`](#) → [`deletePIDAddress`](#) → [`decreaseReferenceCounter`](#)
- [`ProcMunmap`](#) → [`deletePIDAddress`](#) → [`decreaseReferenceCounter`](#)
- [`ProcNewMapping`](#)

With the exception of the reference counter field, entries in this map
are never mutated after insertion.

### The `unwindInfoIndex` map

[The `unwindInfoIndex` map][unwind-info-index] and the `unwind_info_array` BPF
map that it is tethered with provide deduplication of
[`UnwindInfo`][unwind-info-struct] records in the `exe_id_to_%d_stack_deltas`
BPF map. The map in Go assigns each unique `UnwindInfo` the `uint16` index of
the corresponding information in the BPF map. It is both read and written
exclusively from [`getUnwindInfoIndex`][get-unwind-info-func]. Entries are only
appended and then never deleted.

[unwind-info-index]: #
[unwind-info-struct]: #
[get-unwind-info-func]: #

Call-chains:

- [`ProcNewMapping`](#) → [`loadDeltas`](#) → [`getUnwindInfoIndex`](#)
- [`AddSynthIntervalData`](#) → [`loadDeltas`](#) → [`getUnwindInfoIndex`](#)

Because the map is exclusively used as a helper for deduplicating data
in the per-file-ID maps, I suggest moving into the new manager as well.

### The `stackdeltaprovider` field

The [`stackdeltaprovider` field][stackdeltaprovider-field] is used exclusively
to [construct the data that is then stored in the `fileIDInfo`
map][construct-data-for-fileidinfo] and the associated BPF maps. Because the SDP
is concerned only with per-file-ID information, we should move this field to the
new manager as well.

[stackdeltaprovider-field]: #
[construct-data-for-fileidinfo]: #

### The `interpreterLoaders` field

The `interpreterLoaders` field is used to [populate the data][populate-data-1]
in the `fileIDInfo` map if it doesn't exist yet. The field and [the constructor
logic for populating it][populate-data-2] should be moved to the new manager.

[populate-data-1]: #
[populate-data-2]: #

### The `FileIDMapper` field

`FileIDMapper` is a public field of the process manager. It provides
functionality for setting and getting the mapping from user-mode to
kernel-mode file IDs and is [implemented as an LRU](#).
The implementation is swappable to simplify testing.

Entries are read in:

- [`ProcessManager.ConvertTrace`](#)
- [`ebpf.reportNewMapping`](#)

Entries are added in:

- [`ebpf.getKernelStack`](#)
- [`elfinfo.getELFInfoForFile`](#)

Entities are never explicitly deleted. Instead, they are
garbage-collected by falling out of the LRU. It is unclear what the
intended behavior is when binaries that are still loaded in a process
fall out of the LRU. Converting traces for such binaries will no longer
be possible.

The field stores per-file-ID information and should probably be moved as
well. However, being public, swapable for testing, and mutated remotely
complicates this considerably. The actual implementation of the mapper
is already abstracted behind a dedicated type and doesn't introduce a
lot of complexity into the PM code itself, so I suggest leaving it where
it is for the moment. Consolidating it into the new manager might be
desirable at a later point in time but is declared out of scope for this
document.

## Proposed Solution

### New Manager Design

This section makes concrete suggestions for the design of the new manager.

#### Lifetime Management

The two main maps `interpreterData` and `fileIDInfo` discussed in the previous
chapter currently have diverging lifetimes: one is append-only whereas the other
one uses reference counting. In order to merge the two and to make the lifetime
of per-file-ID information easier to reason about, it is desirable to move them
towards a common lifetime. There are multiple possible strategies to pursue
here.

##### All Append-Only

In this strategy we merge both maps and manage them according to an append-only
lifetime: entries that are added once will never be released.

While this strategy worked reasonably well for `interpreterData`, it is doubtful
that it is desirable to also apply it to `fileIDInfo` and the connected BPF
maps: the amount of memory required to store it is significantly higher. On
systems that build and execute a lot of executables on the fly (e.g. wasmtime
runtime with the dylib loader) we'd quickly run out of memory.

##### All Reference-Counted

In this variant, we merge the maps and manage both of them via
reference-counting. While it entails a bit more complexity in the new manager's
internals, it ensures that no memory is being leaked regardless of the workload.

For `fileIDInfo` the lifetime remains unchanged. For the `interpreterData`
information the logic changes slightly when compared to the previous append-only
approach: the per-executable interpreter information is lost once the last
interpreter running it exits. In a workload that spawns a lot of very
short-lived interpreter processes in sequence, this might increase the amount of
cycles spent on re-gathering the interpreter data.

As an optional extension to combat this, we could refrain from removing objects
whose RC reaches 0 immediately. Instead, we store a timestamp of when the RC
reached 0. A goroutine then periodically cleans all entries that have been at RC
= 0 for more than N seconds. Lookup and deletion methods will pretend that
objects with RC = 0 do not exist whereas insertion methods will pick up the
dying object and revive it by incrementing the RC.

It is unclear whether the overhead introduced by these additional
`interpreter.Loader` calls is substantial. `fileIDInfo` has the same lifetime
management already and – from intuition – would appear to be more expensive to
re-create than the interpreter data: it requires reading stack-deltas from disk,
decompressing them and inserting them into the BPF maps.

We could also consider storing the per-executable interpreter information on
disk as well.

##### Keep Maps and Lifetime Separate

This is the simplest variant to implement, but it also reduces the opportunities
for internal simplification significantly. If we keep their lifetime separate,
we'll have to add accessors to read from and write to each respective map
individually.

##### Author's Preference

My preference is managing all information with reference counting. I suggest
going without the delayed deletion approach first to keep things simple. If this
turns out to be a performance problem in practice, it should be reasonably easy
to extend the system as required.

##### Decisions

The reviewers agreed with the author's preference.

#### Public Interface

```go
type ExecutableInfo struct {
  // Data holds per-executable interpreter information if the file ID that this
  // instance belongs to was previously identified as an interpreter. Otherwise,
  // this field is nil.
  Data interpreter.Data
  // Gaps contains information about large gaps in the unwinding information.
  Gaps []libpf.Range
  // [elided: various private fields required to keep track of RC and BPF map entries]
}

type executableInfoManagerState struct {
   info map[FileID]ExecutableInfo
   unwindInfoIndex map[sdtypes.UnwindInfo]uint16
   sdp StackDeltaProvider
   ebpf EbpfHandler
   opener interpreter.ResourceOpener
   numStackDeltaMapPages uint64
}

type ExecutableInfoManager struct {
  state xsync.RWMutex[executableInfoManagerState]
}

// Alias to keep the definitions below short. Not to be included in the actual code.
type EIM = ExecutableInfoManager;

// New creates a new instance of the executable info manager.
func New(sdp StackDeltaProvider, ebpf EbpfHandler, opener interpreter.ResourceOpener) *EIM;

// AddOrIncRef either adds information about an executable to the internal cache (when first
// encountering it) or increments the reference count if the executable is already known.
// The memory mapping and PID are only borrowed in order to allow interpreter loaders to read
// memory (e.g. to determine the version or to resolve other per-executable information).
func (mgr *EIM) AddOrIncRef(mapping *Mapping, pid util.PID) (*ExecutableInfo, error);

// AddSynth adds synthetic interval data to the manager. A call to this function must
// precede all `AddOrIncRef` calls for the same file ID, otherwise an error is returned.
func (mgr *EIM) AddSynth(fileID host.FileID, data sdtypes.IntervalData) error;

// Get retrieves information about an executable. If the executable isn't known, nil is
// returned instead.
func (mgr *EIM) Get(fileID FileID) *ExecutableInfo;

// RemoveOrDecRef decrements the reference counter of the file being tracked. Once the RC
// reaches zero, information about the file is removed from the manager.
func (mgr *EIM) RemoveOrDecRef(fileID FileID) error;

// UpdateMetricSummary updates the metrics in the given map.
func (mgr *EIM) UpdateMetricSummary(summary map[metrics.MetricID]metrics.MetricValue);
```

The suggested interface here assumes that we went with the consolidated
ref-counting approach for lifetime management. Should we decide to go with a
different approach, this will have to be reworked.

The ref-counting semantics are intentionally exposed in the public interface:
callers must understand them; otherwise it might be tempting to ask why not to
only call `Add` once and check existence via `Get`.

All function-, type- and field-names are suggestions and up for debate.

#### Ownership of the New Manager

The instance of the new manager will be owned by the process manager in the form
of a private struct field. The code continues to live in the `processmanager`
package, although in a different file: `execinfomanager.go`.

#### BPF Map Management

The responsibility for managing the following per-file-ID BPF maps is
transferred to the new manager:

- `stack_delta_page_to_info`
- `exe_id_to_%d_stack_deltas`
- `unwind_info_array`

#### BPF Map Documentation

BPF maps pose a bit of a challenge with regards to where and how to document
them. They are always public to all code and their ownership situation can be
intransparent. In essence, they can be reasoned about as if they were public
fields of the type that manages them. Consequently, this might also be a way
forward in documenting them: on the type that manages them, as if they were
public fields.

We should document what with and by whom the maps are populated and whether
other code is allowed to mutate them or not. In this particular case, the maps
are managed exclusively by the new manager and other code can only read from the
maps.

#### Locking

The new manager is locked internally and public methods can be called from an
arbitrary amount of different threads at the same time. A global `xsync.RWMutex`
is used to lock all fields. It is acquired and released in all public functions;
internal helpers that need access to the manager’s state rely on their caller
for locking by taking an `*executableInfoManagerState` argument. If applicable,
they can also be directly implemented as having the state struct as the
receiver.

#### Metrics

The `numStackDeltaMapPages` metric is concerned with keeping count of the size
of per-file-ID BPF maps and should thus be moved to the new manager.  The
responsibility for reporting metric information of various fields that were
moved into the new manager is transferred as well.

#### Moving Code

Multiple methods of the process manager are exclusively concerned with
per-file-ID information:

- `AddSynthIntervalData`
- `loadDeltas`
- `getUnwindInfoIndex` (only called from `loadDeltas`)
- `calculateMergeOpcode` (only called from `loadDeltas`)
- `decreaseReferenceCounter`

These will be moved to the new manager in their entirety.

### Porting Process

#### Option #1

- Copy per-file-ID maps to separate type
- Create methods for all code-snippets that essentially – as far as possible –
  contain exact copies of the code that previously lived in the process manager
    - Will produce a lot of bogus methods with questionable names and semantics
    - It however allows reviewers to easily tell whether code is equivalent
- Remove per-file-ID maps from process manager, replacing them with an instance
  of the new manager
- Replacing the previously extracted code snippets with corresponding method
  calls
- Get that reviewed & merged
- Unify the two maps in the new manager and move towards the new proposed public
  interface
- Get that reviewed & merged

#### Option #2

- Implement a manager with the public interface documented in this document
  immediately
    - Duplicate all required helper functions and fields
    - Place a lot of comments that help explaining where code was pasted from,
      how it was changed and where new code was added
    - Get that reviewed and approved as a draft, but don’t actually merge it
      into `main` yet
- Port PM to use the new manager, replacing and deleting all previously
  duplicated code
- Get that reviewed, approved and then merged

#### Author’s Preference

This is going to be a bit of a challenge to review either way: even when going
with Option #1, there will still be one rather large commit to consolidate the
lifetimes of the two maps and to move towards the new interface. There’s also a
chance that all the bogus methods created to factor out the code snippets from
PM might actually make things harder to review instead of aiding with it,
because reviewers will now also have to check on how these snippet-methods are
called. I thus have a slight preference for Option #2, but will leave it to the
reviewers to decide which variant they prefer.

#### Decision

The reviewers either agreed with the preference for Option #2 or didn’t have a
preference at all. The port will be conducted using Option #2.
