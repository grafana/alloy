symblib
=======

`symblib` is our internal library whose purpose is to parse executables or
shared libraries containing debug information and to turn them into our
simplified symbfile symbol storage format.

## Symbol sources

libpf currently supports the following symbols sources:

| Source               | Function names | Source and line info | Inline info |
|----------------------|----------------|----------------------|-------------|
| ELF dynamic symbols  | yes            | not present          | not present |
| ELF static symbols   | yes            | not present          | not present |
| DWARF                | yes            | yes                  | yes         |
| Go runtime info      | yes            | partial[^1]          | partial[^1] |

[^1]: Extraction supported by our `symblib::gosym` module, but
      `symblib::symbconv::go` doesn't support translating them to our symbfile
      range format yet.

## Design philososphy

Debug symbol formats have a tendency to be very complex. At the same time, they
often force a reader to sift through large amounts of information that is not
relevant for symbolization.

### Debug symbol format parsing abstractions

To keep complexity reasonable, we write abstractions for each such complex format
that allow us to access the relevant portion of this data efficiently. For example,
the [`dwarf`] module provides a wrapper around the excellent but rather low-level
[`gimli`] DWARF parsing library that massively simplifies iterating over the symbols
that DWARF contains.

These abstractions all have the following qualities and design goals:

- **Use zero-copy parsing as much as possible**\
  Not only is this desirable for performance, but it also allows our memory usage
  to largely be independent of the executable size that we are parsing. It's not
  at all uncommon to see 5+ GiB debug build executables these days, and we don't
  want our symbolization tooling memory usage to scale linearly with executable
  size.
- **If allocations are necessary, ensure that they are bounded**\
  Sometimes it is necessary to maintain cache data structures to enable efficient
  parsing. If this is the case, make sure not to trust the executable and ensure
  that every allocation based on sizes from the executable are checked against
  upper bounds. We don't want a broken executable that claims to have MAX_INT
  sections to send us into OOM.
- **If large allocations are necessary, make them using `mmap`ed temp files**\
  Sometimes it's necessary to load executable regions into memory. For example,
  DWARF sections can be compressed in the executable, so we can't just parse them
  from an `mmap`ed executable directly. In these cases, we decompress the data
  into a temporary file and mmap it. This effectively has the performance of a
  regular allocation on a machine with enough memory while at the same time
  behaving gracefully on low-memory machines by simply swapping back and forth
  from disk where necessary.

Examples of such abstractions:

- The [`dwarf`] module abstracts over DWARF internals, exposing lazy, zero-copy
  symbol iterators.
- The [`objfile`] module exposes transparent decompression and relocation of
  executable sections via `mmap`ed temporary files.
- The [`gosym`] module implements parsing for Go's internal runtime structures
  and exposes them through lazy, zero-copy iterators.

### Debugging the debug format parsers

The debug information emitted by modern compilers is not nearly as well-tested
and maintained as the code that they are genearting. It's not at all uncommon to
have at least partially broken debug info. The formats are also often very
complex and it's easy to accidentally do incorrect parsing in our code.

To simplify debugging and investigating such problems, we tend to have an internal
debugging sub-command for each symbol format abstraction in `symbtool`. For
example, the `dwarf` abstration has a corresponding `dwarf` sub-command that in
turn has a `dump` sub-command that prints (and optionally filters) all info that
the format abstraction exposes in a format fit for human consumption.

### Range conversion

All debug symbol formats have their own and often slightly different idea of how
to represent line tables, function name mappings and inline function trees. To
unify the representations, we need to convert these internal representations into
our symbfile format.

For this purpose, we have the [`symbconv::RangeExtractor`] trait and one type
that implements it for each debug symbol format that we support. For example,
the `RangeExtractor` implementation for DWARF symbols is implemented on the
[`symbconv::dwarf::Extractor`] type.

We further have one range extractor implementation that allows merging the output
of multiple other range extractors based on priority and coverage maps:
[`symbconv::multi::Extractor`].

### Return pads

#### Motivation

In a perfect world, the range based symbols would be the only data that we need
for symbolization. However, in practice we have realized that the `_msearch`
queries that are necessary to make the symbol lookups would most likely be too
slow if we did them for every single frame that we want to symbolize.

We then made the following observations:

- in every stack trace, all[^all] frames except for the last one will always be a
  return addresses thats follow a call
- only a small fraction of instructions in a typical executables are calls

Based on these observations, we came up with the idea to:

- let symbtool search all call instructions in the executable
- generate a return pad record with the complete inline-trace for each return
  address that follows the call
- insert these return pads eagerly when the executable is first seen, avoiding
  the need for `_msearch` range queries for 95%+ of frames that we encounter,
  instead allowing us to do much faster `_mget` point lookups

[^all]: Not actually entirely accurate: there are some exceptions like signal
        frames, but they are relatively rare and we currently don't handle them
        correctly.

#### Generation

To generate return pads, we need the following information:

- Range-based symbols as generated by [`symbconv::RangeExtractor`]
- The executable including the code sections. This may sound obvious, but in some
  cases such as split DWARF the executable is split into two files: one with an
  all-zero dummy code section and the debug info (to be stored for debugging)
  and one with the code but without debug info (to be deployed into production).
- Support for the object file format (e.g. ELF) and finding the code sections
  within them
- A [`disas::InstrDecoder`] implementation for the executable's architecture

This generally means that as long as the debug info format that you want to
support is somehow based on regular, native ELF executables on Linux, **you
do not have to worry about this and implementing range extraction should be
sufficient to also get return pad extraction support for free**.

## Error handling design

`symblib` uses strong-typed error handling. Each major sub-module defines their
own `Error` and `Result` types. These types are usually error `enums` that
explicitly list most common problems that can occurr during usage and implement
the [`std::error::Error`] trait via the macros in the [`thiserror`] crate. The
idea here is to allow library users to detect and specifically handle particular
errors. Debug information in real-world executables is often partially broken,
so there's a lot of value to be able to still continue parsing for non-critical
errors.

In some cases where an error is produced by a third party library that we
abstract over, for example by the `object` library used in [`objfile`], errors
that we don't specifically care about are type erased into [`AnyError`]
(`Box<dyn Error>`) to avoid excessive error translation logic or leaking
`object` types from the abstraction.

[`thiserror`]: https://docs.rs/thiserror/latest/thiserror
[`std::error::Error`]: https://doc.rust-lang.org/std/error/trait.Error.html
