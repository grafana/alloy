Custom Labels
=============

# Meta

- **Author(s)**: Tommy Reilly
- **Start Date**: 2025-05-15
- **Goal End Date**: 2025-06-15
- **Primary Reviewers**: Florian Lehner, Timo Teräs, Brennan Vincent

# Problem

Sometimes understanding performance issues is hard because there's no way to dissect hotspots by attributes that aren't visible in the program structure. For instance in a database that uses a generic query execution path to execute all queries you may want to see how much CPU cycles are on behalf of internal queries vs external queries, or you might want to see which user is doing the most queries. This requires attaching metadata to each sample. In Go this is typically done with pprof labels and pprof data can be split out by different values of these labels (example: https://www.polarsignals.com/blog/posts/2021/04/13/demystifying-pprof-labels-with-go).

In addition to pprof labels more examples of where custom labels could be used (out of the box only pprof labels are supported, these theoretical use cases are only intended to help understand the design space better).

- Trace IDs for supporting queries of CPU resources used by a particular traceid
- Runtime metadata like "goid" so that CPU resources associated by a particular Goroutine can be discerned
- Arbitrary application/workload specific metadata like user, client or query

This design doc describes how we can surface Go pprof labels in the OTel profiler and lays the groundwork for doing similar things for other languages but how languages besides Go are supported is beyond the scope of this document.

# Success criteria

- Any native language unwinder should be able to add custom labels to each sample, ie it should not be Go specific even if Go is the initial target
- Custom labels should have its own trace type for enable/disable purposes even though it is technically not an unwinder
- When disabled custom labels has little to no impact on performance or memory usage of the profiler
- Custom labels should be limited so that even if a program has thousands of eligible labels the number supported is reasonably small (mostly enforced by eBPF itself)
- Custom labels should be short and have fixed memory overhead
- The custom labels should be made available to the reporter backend but otherwise it should be left up to implementors what to do with them

# Scope

The initial proposal will only deal with Go pprof labels which are just string/string key/value pairs, more custom labels for Go or other languages may be added in the future. The initial proposal is to get up to 10 labels in best effort fashion, if any eBPF errors occur there may be fewer labels and there is no proposed mechanism for deciding which labels to grab. Even though the OTel proto allows arbitrary types for the value the initial implementation will be scoped to just strings.

# Proposed Solution

The solution we propose is to add support for 10 64 byte custom labels associated with each sample with 16 bytes for the label key and 48 bytes for the label value. These will be stored in the Trace struct with the stack frame information for each sample so each Trace will be 640 bytes larger than before.

In Go 1.23 and lower labels are stored in a map so its non-deterministic which labels are read from the program, in Go 1.24+ the labels are stored in a list sorted by their keys so it will be first come first serve which labels are extracted. If the labels key or value is larger than 16/48 bytes they will be truncated. No effort is made to validate the strings from a UTF8 perspective.
