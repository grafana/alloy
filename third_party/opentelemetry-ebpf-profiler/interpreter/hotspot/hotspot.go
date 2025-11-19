// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package hotspot // import "go.opentelemetry.io/ebpf-profiler/interpreter/hotspot"

// Java HotSpot Unwinder support code (works also with Scala using HotSpot)

//nolint:lll
// The code here and in hotspot_tracer.ebpf.c is based on the Java Serviceability Agent (SA) code,
// and the Java DTrace helper code (libjvm_db). Additional insight is taken from
// https://github.com/jvm-profiling-tools/async-profiler/ unwinding parts, as well as various other
// online resources.
//
// Hotspot libjvm.so provides several tables of introspection data (such as the C++ class field
// offsets, struct sizes, etc.). These tables are accessed using several sets of exported symbols.
// The data from these tables is read and used to introspect the in-process JVM data structures.
// However, some additional assumptions are done e.g. the data type of each field, and the some
// aspects of the Interpreter stack layout. Some documentation about this introspection data is
// available at:
//   https://hg.openjdk.java.net/jdk-updates/jdk14u/file/default/src/hotspot/share/runtime/vmStructs.hpp
//   https://hg.openjdk.java.net/jdk-updates/jdk14u/file/default/src/hotspot/share/runtime/vmStructs.cpp
//
// The main references are available at (libjvm_db.c being the main source):
//   https://hg.openjdk.java.net/jdk-updates/jdk14u/file/default/src/java.base/solaris/native/libjvm_db/libjvm_db.c
//   https://hg.openjdk.java.net/jdk-updates/jdk14u/file/default/src/hotspot/cpu/x86/frame_x86.cpp
//   https://hg.openjdk.java.net/jdk-updates/jdk14u/file/default/src/hotspot/os_cpu/linux_x86/thread_linux_x86.cpp
//   https://hg.openjdk.java.net/jdk-updates/jdk14u/file/default/src/hotspot/share/prims/forte.cpp
//   https://github.com/jvm-profiling-tools/async-profiler/blob/master/src/profiler.cpp#L280
//   https://github.com/jvm-profiling-tools/async-profiler/blob/master/src/stackFrame_x64.cpp
//   https://docs.oracle.com/javase/specs/
//
// In effect, the code here duplicates the Hotspot SA code in simple standalone manner that is
// portable across several JDK versions. Additional handling of certain states is done based
// on external resources, so this code should be faster, more portable and accurate than
// the Hotspot SA, or other tools.
//
// Notes about JDK changes (other than differences in introspection data values) which affect
// unwinding (incomplete list). The list items are changes done between the release major versions.
//
//  JDK7 - Tested ok
//   - renamed multiple C++ class names: methodOopDesc -> Method, constMethodOopDesc -> ConstMethod, etc.
//   - due to the above some pointers are not including the sizeof OopDesc, and need to be explicitly added
//   - InstanceKlass._source_file_name (Symbol*) -> _source_file_name_index
//   - nmethod._oops_offset renamed to _metadata_offset
//  JDK8 - Tested ok
//   - Interpreter stack frame layout changed (BCP offset by one machine word)
//   - CodeBlob: _code_offset separated to _code_begin and _code_end
//   - CodeCache: _heap -> _heaps,  global CodeCache boundaries added
//   - CompiledMethod: split off from nmethod
//   - nmethod._scopes_data_offset -> CompiledMethod._scopes_data_begin
//   - nmethod._method -> CompiledMethod._method
//   - nmethod._deoptimize_offset -> CompiledMethod._deopt_handler_begin
//   - Modules introduced (but no introspection data to access those)
//   - Abstract_VM_Version._vm_security_version exported
//  JDK9 - Tested ok
//  JDK10 - Tested ok
//  JDK11 - Reference, works
//   - Symbol.{_length, _refcount} merged to Symbol._length_and_refcount
//  JDK12 - Tested ok
//   - CompiledMethod smaller, nmethod shifted, works OK
//  JDK13 - Tested ok
//   - nmethod smaller, some members shifted, works OK
//  JDK14 - Tested ok
//   - InstanceKlass.Source_file_name_index moved to ConstantPool
//  JDK15 - Tested ok
//   - GenericGrowableArray renamed to GrowableArrayBase
//  JDK16 - Tested ok
//  JDK17 - Tested ok
//  JDK18 - Tested ok
//  JDK19 - Tested ok
//   - Excluding zero byte from UNSIGNED5 encoding output
//  JDK20 - Tested ok
//  JDK21 - Tested ok
//   - JDK_Version removed from introspection data
//  JDK22 - Tested ok
//   - nmethod scopes etc. immutable data split to separate data area
//  JDK23 - Tested ok
//  JDK24 - Tested ok
//   - nmethod metadata moved to codeblob mutable data area
//  JDK25 - Tested ok
//
// NOTE: Ahead-Of-Time compilation (AOT) is NOT SUPPORTED. The main complication is that, the AOT
// ELF files are mapped directly to the program virtual space, and contain the code to execute.
// This causes the functions to not be in the Java CodeCache and be invisible for the unwinder.
//
// NOTE: Compressed oops is a feature of HotSpot that reduces the size of pointers to Java objects
// on the heap in many cases (see https://wiki.openjdk.org/display/HotSpot/CompressedOops for more
// details). It is turned on by default for heap sizes smaller than 32GB. However, the unwinder
// is not affected by compressed oops as we don't process objects on the Java heap.
//
// The approach is that code here will read the introspection data tables once for each JVM file
// and configure the important field offsets to eBPF via HotspotProcInfo. The eBPF code will then
// have enough data to unwind any HotSpot function via the JVM debug data. On each frame, the eBPF
// code will store four variables: 1. frame subtype, 2. a pointer, 3. a cookie, and 4. rip/bcp
// delta. The pointer is nmethod* (for JIT frames) or Method* (for Interpreted frames).
//
// Once the frame is received in this module's Symbolize function, the code will read additional
// data from the target process at given pointer. The "cookie" is also used to verify that the data
// at given pointer is still describing the same method as during the eBPF capture time. The read
// data is then further parsed, pointers there are followed by reading additional data as needed.
// Caching is done where possible to avoid reads from the target process via syscalls to reduce CPU
// overhead. Finally there should be enough data to produce symbolized frames.
//
// The above approach is selected because the process of symbolizing a JVM Method requires unbounded
// loops to parse the lineNumber tables and cannot be done in the eBPF code. The implication is that
// the frame values are specific to the process (as it has a pointer in it). Meaning the Trace IDs
// will be different if there's multiple VM instances running same Java application. But the overhead
// is not huge here. This also has the implication that in the very unlikely even of two different
// JVM instances producing identical trace (highly unlikely due to ASLR and the address cookie) the
// count aggregation might incorrectly produce wrong expansion. However, it's more likely that there
// will be trace hash collision due to other factors where the same issue would happen.

import (
	"regexp"

	log "github.com/sirupsen/logrus"

	"go.opentelemetry.io/ebpf-profiler/interpreter"
)

var (
	// The following regex is intended to match the HotSpot libjvm.so
	libjvmRegex = regexp.MustCompile(`.*/libjvm\.so`)

	// Match Java Hidden Class identifier and the replacement string
	hiddenClassRegex = regexp.MustCompile(`\+0x[0-9a-f]{16}`)
	hiddenClassMask  = "+<hidden>"

	_ interpreter.Data     = &hotspotData{}
	_ interpreter.Instance = &hotspotInstance{}
)

// Loader is the main function for ProcessManager to recognize and hook the HotSpot
// libjvm for enabling JVM unwinding and symbolization.
func Loader(_ interpreter.EbpfHandler, info *interpreter.LoaderInfo) (interpreter.Data, error) {
	if !libjvmRegex.MatchString(info.FileName()) {
		return nil, nil
	}

	log.Debugf("HotSpot inspecting %v", info.FileName())
	ef, err := info.GetELF()
	if err != nil {
		return nil, err
	}
	return newHotspotData(info.FileName(), ef)
}
