# Introduction

This repository implements a whole-system, cross-language profiler for Linux via
eBPF.

## Core features and strengths

- Implements the [experimental OTel profiling
  signal](https://github.com/open-telemetry/opentelemetry-proto/pull/534)
- Very low CPU and memory overhead (1% CPU and 250MB memory are our upper limits
  in testing and the agent typically manages to stay way below that)
- Support for native C/C++ executables without the need for DWARF debug
  information (by leveraging `.eh_frame` data as described in
  [US11604718B1](https://patents.google.com/patent/US11604718B1/en?inventor=thomas+dullien&oq=thomas+dullien))
- Support profiling of system libraries **without frame pointers** and **without
  debug symbols on the host**.
- Support for mixed stacktraces between runtimes - stacktraces go from Kernel
  space through unmodified system libraries all the way into high-level
  languages.
- Support for native code (C/C++, Rust, Zig, Go, etc. without debug symbols on
  host)
- Support for a broad set of HLLs (Hotspot JVM, Python, Ruby, PHP, Node.JS, V8,
  Perl), .NET is in preparation.
- 100% non-intrusive: there's no need to load agents or libraries into the
  processes that are being profiled.
- No need for any reconfiguration, instrumentation or restarts of HLL
  interpreters and VMs: the agent supports unwinding each of the supported
  languages in the default configuration.
- ARM64 support for all unwinders except NodeJS.
- Support for native `inline frames`, which provide insights into compiler
  optimizations and offer a higher precision of function call chains.

## Building

We are working towards integrating the profiling functionality into the [OTel Collector](https://opentelemetry.io/docs/collector/) as a receiver,
which will be the supported configuration going forward. In the meantime, we also offer a standalone profiling agent binary named `ebpf-profiler`,
to aid with development and debugging. The expectation is that this will go away once the integration with the [OTel Collector](https://opentelemetry.io/docs/collector/) is complete.

## Platform Requirements
The agent can be built with the provided make targets. Docker is required for containerized builds, and both amd64 and arm64 architectures are supported.

 For **Linux**, the following steps apply:
  1. Build the agent for your current machine's architecture:
     ```sh
     make agent
     ```
     Or `make debug-agent` for debug build.
  2. To cross-compile for a different architecture (e.g. arm64):
     ```sh
     make agent TARGET_ARCH=arm64
     ```
The resulting binary will be named `ebpf-profiler` in the current directory.

## Other OSes
Since the profiler is Linux-only, macOS and Windows users need to set up a Linux VM to build and run the agent. Ensure the appropriate architecture is specified if using cross-compilation. Use the same make targets as above after the Linux environment is configured in the VM.

## Supported Linux kernel version

[7ddc23ea](https://github.com/open-telemetry/opentelemetry-ebpf-profiler/commit/7ddc23ea135a2e00fffc17850ab90534e9b63108) is the last commit with support for 4.19. Changes after this commit may require a minimal Linux kernel version of 5.4.

## Alternative Build (Without Docker)
You can build the agent without Docker by directly installing the dependencies listed in the Dockerfile. Once dependencies are set up, simply run:
```sh
make
```
or
```sh
make debug
```
This will build the profiler natively on your machine.

## Running

You can start the agent with the following command:

```sh
sudo ./ebpf-profiler -collection-agent=127.0.0.1:11000 -disable-tls
```

The agent comes with a functional but work-in-progress / evolving implementation
of the recently released OTel profiling [signal](https://github.com/open-telemetry/opentelemetry-proto/pull/534).

The agent loads the eBPF program and its maps, starts unwinding and reports
captured traces to the backend.

## Development

To understand how this project works and learn more about profiling, check out [Profiling internals](doc/internals.md)


# Legal

## Licensing Information

This project is licensed under the Apache License 2.0 (Apache-2.0).
[Apache License 2.0](LICENSE)

The eBPF source code is licensed under the GPL 2.0 license.
[GPL 2.0](support/ebpf/LICENSE)

## Licenses of dependencies

To display a summary of the dependencies' licenses:
```sh
make legal
```
