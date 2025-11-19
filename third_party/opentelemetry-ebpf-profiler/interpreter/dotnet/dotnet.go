// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dotnet // import "go.opentelemetry.io/ebpf-profiler/interpreter/dotnet"

// Microsoft .Net Unwinder support code

// The Microsoft dotnet is formally specified in ECMA-335. For the main references see:
//nolint:lll
// sources  https://github.com/dotnet/runtime/
// ECMA-335 https://www.ecma-international.org/wp-content/uploads/ECMA-335_6th_edition_june_2012.pdf
// R2RFMT   https://github.com/dotnet/runtime/blob/v8.0.0/docs/design/coreclr/botr/readytorun-format.md

// The dotnet runtime sources uses specific termionology. And there is also documentation
// about the internals. Find below useful resources to get started with the runtime:
//  https://github.com/dotnet/runtime/blob/v8.0.0/docs/project/glossary.md
//  https://github.com/dotnet/runtime/blob/v8.0.0/docs/design/coreclr/botr/README.md
//  https://github.com/dotnet/runtime/blob/v8.0.0/docs/design/coreclr/botr/clr-abi.md
//  https://github.com/dotnet/runtime/tree/v8.0.0/docs/design/specs

// To understand the ECMA specification better, the following blog series was useful:
//  https://www.red-gate.com/simple-talk/blogs/anatomy-of-a-net-assembly-pe-headers/
//  https://www.red-gate.com/simple-talk/blogs/anatomy-of-a-net-assembly-clr-metadata-1/
//  https://www.red-gate.com/simple-talk/blogs/anatomy-of-a-net-assembly-clr-metadata-2/
//  https://www.red-gate.com/simple-talk/blogs/anatomy-of-a-net-assembly-clr-metadata-3/
//  https://www.red-gate.com/simple-talk/blogs/anatomy-of-a-net-assembly-methods/

// On Windows x64, the standard Windows error handling is used. Originally, this was also
// used on Linux. The complicated details are at:
// https://docs.microsoft.com/en-us/cpp/build/exception-handling-x64?view=msvc-160
// Fortunately, since dotnet5 this was adjusted that frame pointer frame chains are always
// preserved on Linux. See: https://github.com/dotnet/runtime/issues/4651

// Dotnet core itself implements stack unwinding of remote dotnet process by something
// called "Data Access Component" or DAC. It is build of the dotnet vm in special mode
// (macro DACCESS_COMPILE is defined) where it supports directly reading the classes/structures
// from remote process. The DAC DSO needs to always match the dotnet VM of the target process.
// So e.g. on Windows it can automatically detect the version on attach, and even download
// the DAC from Microsoft. Though, typically on every dotnet framework install the DAC is
// also installed along with the dotnet VM.
// Due to this approach, there is no real introspection data available. There has been some
// talk to have an "Universal DAC" where one .DLL could support any target VM. If this gets
// implemented, the dotnet VM will likely start shipping introspection data we can use also.

// Our strategy currently is as follows:
//  1. inspect the dotnet to find the mmapped PE DLL executable code and the JIT areas along
//     with its metadata maps. insert this into ebpf pid_page mappings.
//  2. just use standard frame pointer unwinding in ebpf, and also locate the JIT function
//     code header to get access to debug data and method descriptors
//  3. in the host agent, the debug data and method descriptors are resolved and mapped to
//     PE (.dll) FileID, Method index, and the IL code offset (JIT code), or the PE FileID
//     and Relative Virtual Address (RVA) (Ready to Run code)
//  4. symbolizer can then map the above data to source code file and line
//  5. we explicitly do not support debug features such as Edit-and-Continue, or other
//     external manipulation to the files (e.g. via ICorProfile API)

// The dotnet runtime has had various features come and go, and this explains some of them.
// NGEN        Native Image Generator. Conceptually this was intended to be run on the server
//             executing the code to pre-compile everything (not as part of build, but as part
//             of deployment). It is now removed from the code base since dotnet7.
// R2R         Ready to Run. This feature embeds precompiled native code along with the IL
//             to the PE .dll files during build. It is intended to reduce startup overhead.
//             R2R compiled methods often will still be replaced by JIT optimized methods.
// NativeAOT   Native AOT will build statically linked native executable. It includes all
//             dotnet code, coreclr and the runtime. Everything is statically compiled and
//             there is no JIT nor IL code included in the executable. Also dotnet framework
//             is not needed to run this binary. There are various limitations on what kind
//             of code is supported. Currently not supported by this interpreter code.

// The dotnet runtime has numerous compile-time FEATURE_* macros to enable or disable
// some specific functionality. Fortunately, practically all of them are fixed in the build
// based on CPU architecture, OS and dotnet version. This code assumes official build with
// the default build-time featureset, but comments are added where such assumptions are made.

// We currently make the following build feature assumptions:
// _DEBUG                        Is assumed off. High overhead. Not enabled on production builds.
// FEATURE_GDBJIT                Is assumed off. It has high overhead. And is enabled on some
//                               specific custom builds only.
// FEATURE_PREJIT                Is assumed off. It was specific to NGEN support, and was not
//                               enabled on builds we support.
// FEATURE_ON_STACK_REPLACEMENT  Assume on. Enabled always on x64 and arm64.

// Additional references:
//   https://mattwarren.org/2019/01/21/Stackwalking-in-the-.NET-Runtime/
//   https://github.com/dotnet/diagnostics/tree/v8.0.505301/documentation
//   https://github.com/dotnet/runtime/blob/v8.0.0/docs/design/specs/PortablePdb-Metadata.md
//   https://github.com/dotnet/runtime/blob/v8.0.0/docs/design/coreclr/botr/readytorun-format.md

// Known issues (due to dotnet runtime limitations):
// - Large methods are not handled correctly (and fail to symbolize)
//   see: https://github.com/dotnet/runtime/issues/93550
// - Inlining information is not available in default configuration
//   see: https://github.com/dotnet/runtime/issues/96473
// - Line numbers for Release built modules are inaccurate
//   see: https://github.com/dotnet/runtime/issues/96473#issuecomment-1890383639

// TODO:
// - more coredump testcases
// - support On-Stack-Replacement (OSR): if OSR happened, currently a duplicate frame is shown
// - support for loaded IL code without backing PE DLL: Reflection.Emit, Assembly.Load(byte[])

import (
	"fmt"
	"regexp"
	"strconv"

	log "github.com/sirupsen/logrus"

	"go.opentelemetry.io/ebpf-profiler/interpreter"
)

const (
	// dumpDebugInfo enables extra analysis for debug info.
	// Useful for development and bug finding. But the amount of logs
	// is so much that this is to be enabled on developer build if needed.
	dumpDebugInfo = false

	// maxBoundsSize is the maximum size of boundary debug info (for a method)
	// that we accept as valid. This is to prevent OOM situation.
	maxBoundsSize = 16 * 1024
)

var (
	// regex for the core language runtime
	dotnetRegex = regexp.MustCompile(`/(\d+)\.(\d+).(\d+)/libcoreclr.so$`)

	// compiler check to make sure the needed interfaces are satisfied
	_ interpreter.Data     = &dotnetData{}
	_ interpreter.Instance = &dotnetInstance{}
)

// dotnetVer encodes the x.y.z version to single uint32
func dotnetVer(x, y, z uint32) uint32 {
	return (x << 24) + (y << 16) + z
}

func Loader(_ interpreter.EbpfHandler, info *interpreter.LoaderInfo) (interpreter.Data, error) {
	// The dotnet DSOs are in a directory with the version such as:
	// /usr/lib/dotnet/shared/Microsoft.NETCore.App/6.0.25/libcoreclr.so
	// It is possible to find the version also from the .so itself, but
	// it requires loading the RODATA and doing a string search.
	matches := dotnetRegex.FindStringSubmatch(info.FileName())
	if matches == nil {
		return nil, nil
	}
	major, _ := strconv.Atoi(matches[1])
	minor, _ := strconv.Atoi(matches[2])
	release, _ := strconv.Atoi(matches[3])
	version := dotnetVer(uint32(major), uint32(minor), uint32(release))

	// dotnet8 requires additional support for RangeSectionMap and MethodDesc updates
	if version < dotnetVer(6, 0, 0) || version >= dotnetVer(9, 0, 0) {
		return nil, fmt.Errorf("dotnet version %d.%d.%d not supported",
			major, minor, release)
	}

	ef, err := info.GetELF()
	if err != nil {
		return nil, err
	}

	addr, err := ef.LookupSymbolAddress("g_dacTable")
	if err != nil {
		return nil, err
	}

	log.Debugf("Dotnet DAC table at %x", addr)

	d := &dotnetData{
		version:      version,
		dacTableAddr: addr,
	}
	d.loadIntrospectionData()

	return d, nil
}
