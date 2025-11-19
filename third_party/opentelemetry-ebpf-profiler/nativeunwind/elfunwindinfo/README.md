# Extracting stack deltas from DWARF

The code in this directory is responsible for extracting stack deltas from the
DWARF information in executables or their debug symbols.

## What are stack deltas?

In order to unwind native stack frames if the frame pointer has been omitted, it
is necessary to locate the "top of the function frame" when given a RIP. This is
usually at some offset from the current stack pointer RSP - the stack pointer can
vary throughout the function by pushing arguments to called functions etc.

In short, for every RIP in a given binary, there is an associated value that can
provide the "top" of the function frame, e.g. the address where the return
address is stored, if added to RSP. We call this value 'stack delta'.

## From where can we obtain stack deltas?

The "safest" way to obtain them would be to perform a full disassembly and then
to track the stack pointer accordingly. This would deal with hand-written code
where the compiler cannot generate debug information etc.

This is too time-consuming to develop at the moment. As a stopgap, it turns out
that modern ELF executables (e.g. those compiled in the last few years) have all
the necessary information to obtain the stack deltas in the `.eh_frame` section;
it is placed there to enable stack unwinding for C++ exceptions. This is very
useful for us.

## How do we obtain stack deltas from the `.eh_frame` section?

The section is in almost the same format as the `.debug_frame` section in the
debugging symbols. The DWARF format is optimized to both minimize the necessary
storage as well as supporting 20+ different CPU architectures; the solution in
DWARF is a fairly involved bytecode format and a small VM that allows the
calculation of the stack delta given an address.

Fortunately the `.eh_frame` can contain only a fraction of DWARF commands, so
we implement that ourselves to parse efficiently the stack deltas, and other
needed information such RBP location in CFA to recover it.

## Future work?

The `.eh_frame` section is often buggy, not all compilers generate it, and there
are many other problems with it. The approach (disassemble & reconstruct)
discussed above was implemented by researchers; the following presentation gives
a good overview of the challenges and problems of working with DWARF (as well
as references to papers that validate unwind tables etc.)

https://entropy2018.sciencesconf.org/data/nardelli.pdf

