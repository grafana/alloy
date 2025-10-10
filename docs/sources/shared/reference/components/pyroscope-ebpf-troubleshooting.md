---
canonical: https://grafana.com/docs/alloy/latest/shared/reference/components/pyroscope-ebpf/troubleshooting/
description: Learn how to troubleshoot and resolve eBPF collection issues.
headless: true
---

Learn how to troubleshoot and resolve eBPF collection issues.

### Supported higher level languages

Profiling higher level languages is possible on Alloy versions later than v1.11, as from that version the [Open-Telemetry eBPF profiler] is used for `pyroscope.ebpf`.

Hence the profiler has support for:

- Hotspot JVM
- Node.JS (v8)
- PHP
- Perl
- Python
- Ruby
- .NET (on `amd64` architecture only)

[Open-Telemetry eBPF profiler]:https://github.com/open-telemetry/opentelemetry-ebpf-profiler

### Troubleshoot unknown symbols

Symbols are extracted from various sources, including:

* The `.symtab` and `.dynsym` sections in the ELF file.
* The `.symtab` and `.dynsym` sections in the debug ELF file.
* The `.gopclntab` section in Go language ELF files.

The search for debug files follows [gdb algorithm](https://sourceware.org/gdb/onlinedocs/gdb/Separate-Debug-Files.html).
For example, if the profiler wants to find the debug file
for `/lib/x86_64-linux-gnu/libc.so.6`
with a `.gnu_debuglink` set to `libc.so.6.debug` and a build ID `0123456789abcdef`. The following paths are examined:

* `/usr/lib/debug/.build-id/01/0123456789abcdef.debug`
* `/lib/x86_64-linux-gnu/libc.so.6.debug`
* `/lib/x86_64-linux-gnu/.debug/libc.so.6.debug`
* `/usr/lib/debug/lib/x86_64-linux-gnu/libc.so.6.debug`

#### Deal with unknown symbols

Unknown symbols in the profiles you’ve collected indicate that the profiler couldn't access an ELF file associated with a given address in the trace.

This can occur for several reasons:

* The process has terminated, making the ELF file inaccessible.
* The ELF file is either corrupted or not recognized as an ELF file.
* There is no corresponding ELF file entry in `/proc/pid/maps` for the address in the stack trace.

#### Address unresolved symbols

If you only see module names (e.g., `/lib/x86_64-linux-gnu/libc.so.6`) without corresponding function names, this
indicates that the symbols couldn't be mapped to their respective function names.

This can occur for several reasons:

* The binary has been stripped, leaving no `.symtab`, `.dynsym`, or `.gopclntab` sections in the ELF file.
* The debug file is missing or could not be located.

To fix this for your binaries, ensure that they are either not stripped or that you have separate
debug files available. You can achieve this by running:

```bash
objcopy --only-keep-debug elf elf.debug
strip elf -o elf.stripped
objcopy --add-gnu-debuglink=elf.debug elf.stripped elf.debuglink
```

For system libraries, ensure that debug symbols are installed. On Ubuntu, for example, you can install debug symbols
for `libc` by executing:

```bash
apt install libc6-dbg
```

#### Understand flat stack traces

If your profiles show many shallow stack traces, typically 1-2 frames deep, your binary might have been compiled without frame pointers.

To compile your code with frame pointers, include the `-fno-omit-frame-pointer` flag in your compiler options.


### Troubleshoot missing Python frames

#### Ensure Python interpreter is discoverable

This can happen when `pyroscope.ebpf` cannot locate required Python runtime symbols, potentially due to nonstandard binary/library file naming:

In order to be supported it needs pass the following regular expressions [source](https://github.com/grafana/opentelemetry-ebpf-profiler/blob/c80acf3265fe868d107fe40e319ec144cf2983a7/interpreter/python/python.go#L42-L47)

```go
// The following regexs are intended to match either a path to a Python binary or
// library.
var (
	pythonRegex    = regexp.MustCompile(`^(?:.*/)?python(\d)\.(\d+)(d|m|dm)?$`)
	libpythonRegex = regexp.MustCompile(`^(?:.*/)?libpython(\d)\.(\d+)[^/]*`)
)
```

This means that for example a library named `libpython3.10.so.1.0`, would match this, although `libpython3-custom.10.so.1.0` would not.

To resolve this, ensure Python libraries follow standard naming conventions.

