// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package perl // import "go.opentelemetry.io/ebpf-profiler/interpreter/perl"

// Perl interpreter unwinder

// Start by reading the 'perlguts illustrated' it is really helpful on explaining
// the Perl VM internal implementation:
//   https://github.com/rurban/illguts/blob/master/illguts.pdf
//
// Additional recommended reading from the Perl manuals:
//   https://perldoc.perl.org/perlguts#Dynamic-Scope-and-the-Context-Stack
//   https://perldoc.perl.org/perlinterp#OP-TREES
//   https://perldoc.perl.org/perlcall
//   https://perldoc.perl.org/perlxs

// It is said that reading Perl code makes your eyes bleed.
// I say reading Perl interpreter code with all the unsafe casting,
// and unions makes your brains bleed. -tt

// Perl uses a SV (Scalar Value) as the base "variant" type for all Perl VM
// variables. It can be a one of various different types, but we are mostly
// interested about CV (Code Value), GV (Glob Value, aka. symbol name), and
// HV (Hash Value). Typically the only difference between e.g. SV and CV is
// only that the pointer is of different types, and casts between these structs
// are done if the type code shows the cast is ok.
//
// Much of the extra data is behind the "any" or "variant" pointer. Typically
// named XPVxV (where 'x' changes, so XPVCV for CV). Other types may have another
// additional pointer in the base 'SV' too, like the HV and GV. Bulk of the code
// is just following these pointers (ensuring right types). Please refer to the
// Perlguts illustrated for the relationships.
//
// See the perl_tracer.ebpf.c for more detailed unwinding explanation.
// The tracer will send the 'EGV' (effective GV, aka canonical symbol name) and
// the 'COP' for each frame. This code will stringify the EGV to a full qualified
// symbol name, and extract the source file/line from the COP. The EGV is null for
// the bottom frame which is the global file scope (not inside a function).
//
// Unfortunately, it is not possible to extract file/line where some function is
// defined. The observant may note that the 'struct gp' which is the symbol definition
// holds source file name and line number of its "first definition". But this refers
// either to the closing '}' of the sub definition, or the line of the object
// reference creation ... both of which are not useful for us. It really seems to
// not be possible to get a function's start line.

import (
	"debug/elf"
	"regexp"
	"slices"

	"go.opentelemetry.io/ebpf-profiler/interpreter"
)

const (
	// Scalar Value types (SVt)
	// https://github.com/Perl/perl5/blob/v5.32.0/sv.h#L132-L166
	SVt_MASK uint32 = 0x1f
	SVt_PVHV uint32 = 12

	// Arbitrary string length limit to make sure we don't panic with out-of-memory
	hekLenLimit = 0x10000
)

var (
	// regex for the interpreter executable
	perlRegex    = regexp.MustCompile(`^(?:.*/)?perl$`)
	libperlRegex = regexp.MustCompile(`^(?:.*/)?libperl\.so[^/]*$`)

	// compiler check to make sure the needed interfaces are satisfied
	_ interpreter.Data     = &perlData{}
	_ interpreter.Instance = &perlInstance{}
)

// perlVersion revision, version and subversion to a single uitn32 with the full version
func perlVersion(revision, version, subversion byte) uint32 {
	return uint32(revision)*0x10000 + uint32(version)*0x100 + uint32(subversion)
}

func Loader(ebpf interpreter.EbpfHandler, info *interpreter.LoaderInfo) (interpreter.Data, error) {
	mainDSO := false
	if !libperlRegex.MatchString(info.FileName()) {
		mainDSO = true
		if !perlRegex.MatchString(info.FileName()) {
			return nil, nil
		}
	}

	ef, err := info.GetELF()
	if err != nil {
		return nil, err
	}

	if mainDSO {
		var needed []string
		needed, err = ef.DynString(elf.DT_NEEDED)
		if err != nil {
			return nil, err
		}
		if slices.ContainsFunc(needed, libperlRegex.MatchString) {
			// 'perl' linked with 'libperl'. The beef is in the library,
			// so do not try to inspect the shim main binary.
			return nil, nil
		}
	}

	return newData(ebpf, info, ef)
}
