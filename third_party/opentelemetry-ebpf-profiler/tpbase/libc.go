// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tpbase // import "go.opentelemetry.io/ebpf-profiler/tpbase"

import (
	"debug/elf"
	"fmt"
	"regexp"

	"go.opentelemetry.io/ebpf-profiler/libpf/pfelf"
)

// TSDInfo contains information to access C-library's Thread Specific Data from eBPF
type TSDInfo struct {
	// Offset is the pointer difference from "tpbase" pointer to the C-library
	// specific struct pthread's member containing the thread specific data:
	// .tsd (musl) or .specific (glibc).
	// Note: on x86_64 it's positive value, and arm64 it is negative value as
	// "tpbase" register has different purpose and pointer value per platform ABI.
	Offset int16

	// Multiplier is the TSD specific value array element size.
	// Typically 8 bytes on 64bit musl and 16 bytes on 64bit glibc
	Multiplier uint8

	// Indirect is a flag indicating if the "tpbase + Offset" points to a member
	// which is a pointer the array (musl) and not the array itself (glibc).
	Indirect uint8
}

// This code analyzes the C-library provided POSIX defined function which is used
// to read thread-specific data (TSD):
//   void *pthread_getspecific(pthread_key_t key);
//
// The actual symbol and its location is C-library specific:
//
// LIBC			DSO			Symbol
// musl/alpine		ld-musl-$ARCH.so.1	pthread_getspecific
// musl/generic		libc.musl-$ARCH.so.1	pthread_getspecific
// glibc/new		libc.so.6		__pthread_getspecific
// glibc/old		libpthread.so.0		__pthread_getspecific

// musl:
// http://git.musl-libc.org/cgit/musl/tree/src/internal/pthread_impl.h?h=v1.2.3#n49
// http://git.musl-libc.org/cgit/musl/tree/src/thread/pthread_getspecific.c?h=v1.2.3#n4
//
// struct pthread {
//   ...
//   void **tsd;
//   ...
// };
//
// The implementation is just "return self->tsd[key];". We do the same.

// glibc:
// https://sourceware.org/git/?p=glibc.git;a=blob;f=nptl/descr.h;hb=c804cd1c00ad#l307
// https://sourceware.org/git/?p=glibc.git;a=blob;f=nptl/pthread_getspecific.c;hb=c804cd1c00ad#l23
//
// struct pthread {
//   ...
//   struct pthread_key_data {
//     uintptr_t seq;
//     void *data;
//   } specific_1stblock[PTHREAD_KEY_2NDLEVEL_SIZE];
//   struct pthread_key_data *specific[PTHREAD_KEY_1STLEVEL_SIZE];
//   ...
// }
//
// The 1st block is special cased for keys smaller than PTHREAD_KEY_2NDLEVEL_SIZE.
// We also assume we don't see large keys, and support only the small key case.
// Further both x86_64 and arm64 disassembler assume that small key code is the
// main code flow (as in, any conditional jumps are not followed).
//
// Reading the value is basically "return self->specific_1stblock[key].data;"

var (
	// regex for the libc
	libcRegex = regexp.MustCompile(`.*/(ld-musl|libc|libpthread)([-.].*)?\.so`)
)

// IsPotentialTSDDSO determines if the DSO filename potentially contains pthread code
func IsPotentialTSDDSO(filename string) bool {
	return libcRegex.MatchString(filename)
}

// ExtractTSDInfo extracts the introspection data for pthread thread specific data.
func ExtractTSDInfo(ef *pfelf.File) (*TSDInfo, error) {
	_, code, err := ef.SymbolData("__pthread_getspecific", 2048)
	if err != nil {
		_, code, err = ef.SymbolData("pthread_getspecific", 2048)
		if err != nil {
			return nil, fmt.Errorf("unable to read 'pthread_getspecific': %s", err)
		}
	}
	if len(code) < 8 {
		return nil, fmt.Errorf("getspecific function size is %d", len(code))
	}

	var info TSDInfo
	switch ef.Machine {
	case elf.EM_AARCH64:
		info, err = extractTSDInfoARM(code)
	case elf.EM_X86_64:
		info, err = extractTSDInfoX86(code)
	default:
		return nil, fmt.Errorf("unsupported arch %s", ef.Machine.String())
	}
	if err != nil {
		return nil, fmt.Errorf("failed to extract getspecific data: %s", err)
	}
	return &info, nil
}
