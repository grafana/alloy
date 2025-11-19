// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package perl // import "go.opentelemetry.io/ebpf-profiler/interpreter/perl"

import (
	"fmt"

	"github.com/elastic/go-freelru"
	log "github.com/sirupsen/logrus"

	"go.opentelemetry.io/ebpf-profiler/interpreter"
	"go.opentelemetry.io/ebpf-profiler/libpf"
	"go.opentelemetry.io/ebpf-profiler/libpf/pfelf"
	"go.opentelemetry.io/ebpf-profiler/remotememory"
	"go.opentelemetry.io/ebpf-profiler/support"
)

type perlData struct {
	// vmStructs reflects the Perl internal class names and the offsets of named field
	// The struct names are based on the Perl C "struct name", the alternate typedef seen
	// mostly in code is in parenthesis.
	vmStructs struct {
		// interpreter struct (PerlInterpreter) is defined in intrpvar.h via macro trickery
		// https://github.com/Perl/perl5/blob/v5.32.0/intrpvar.h
		interpreter struct {
			curcop       uint
			curstackinfo uint
		}
		// stackinfo struct (PERL_SI) is defined in cop.h
		// https://github.com/Perl/perl5/blob/v5.32.0/cop.h#L1037-L1055
		stackinfo struct {
			si_cxstack uint
			si_next    uint
			si_cxix    uint
			si_type    uint
		}
		// context struct (PERL_CONTEXT) is defined in cop.h
		// https://github.com/Perl/perl5/blob/v5.32.0/cop.h#L878-L884
		context struct {
			cx_type       uint
			blk_oldcop    uint
			blk_sub_retop uint
			blk_sub_cv    uint
			sizeof        uint
		}
		// cop struct (COP), a "control op" is defined in cop.h
		// https://github.com/Perl/perl5/blob/v5.32.0/cop.h#L397-L424
		cop struct {
			cop_line uint
			cop_file uint
			sizeof   uint
		}
		// sv struct (SV) is "Scalar Value", the generic "base" for all
		// perl variants, and is horrendously cast to other types as needed.
		// https://github.com/Perl/perl5/blob/v5.32.0/sv.h#L233-L236
		sv struct {
			sv_any   uint
			sv_flags uint
			svu_gp   uint
			svu_hash uint
			sizeof   uint
		}
		// xpvcv struct (XPVCV) is "Code Value object" (the data PV points to)
		// https://github.com/Perl/perl5/blob/v5.32.0/cv.h#L13-L16
		xpvcv struct {
			xcv_flags uint
			xcv_gv    uint
		}
		// xpvgv struct (XPVGV) is "Glob Value object" (the data GV points to)
		// https://github.com/Perl/perl5/blob/v5.32.0/sv.h#L571-L575
		xpvgv struct {
			xivu_namehek uint
			xgv_stash    uint
		}
		// xpvhv struct (XPVHV) is a "Hash Value" (that is the Hash struct)
		// https://github.com/Perl/perl5/blob/v5.32.0/hv.h#L135-L140
		xpvhv struct {
			xhv_max uint
		}
		// xpvhv_with_aux is the successor of XPVHV starting in Perl 5.36.
		// https://github.com/Perl/perl5/blob/v5.36.0/hv.h#L149-L155
		xpvhv_with_aux struct {
			xpvhv_aux uint
		}
		// xpvhv_aux struct is the Hash ancillary data structure
		// https://github.com/Perl/perl5/blob/v5.32.0/hv.h#L108-L128
		xpvhv_aux struct {
			xhv_name_u     uint
			xhv_name_count uint
			sizeof         uint
			pointer_size   uint
		}
		// gp struct (GP) is apparently "Glob Private", essentially a function definition
		// https://github.com/Perl/perl5/blob/v5.32.0/gv.h#L11-L24
		gp struct {
			gp_egv uint
		}
		// hek struct (HEK) is "Hash Entry Key", a hash/len/key triplet
		// https://github.com/Perl/perl5/blob/v5.32.0/hv.h#L44-L57
		hek struct {
			hek_len uint
			hek_key uint
		}
	}

	// stateAddr is the address of the Perl state address (TSD or global)
	stateAddr libpf.SymbolValue

	// version contains the Perl version
	version uint32

	// stateInTSD is set if the we have state TSD key address
	stateInTSD bool
}

func (d *perlData) String() string {
	ver := d.version
	return fmt.Sprintf("Perl %d.%d.%d", (ver>>16)&0xff, (ver>>8)&0xff, ver&0xff)
}

func (d *perlData) Attach(_ interpreter.EbpfHandler, _ libpf.PID, bias libpf.Address,
	rm remotememory.RemoteMemory) (interpreter.Instance, error) {
	addrToHEK, err := freelru.New[libpf.Address, libpf.String](interpreter.LruFunctionCacheSize,
		libpf.Address.Hash32)
	if err != nil {
		return nil, err
	}

	addrToCOP, err := freelru.New[copKey, *perlCOP](interpreter.LruFunctionCacheSize*8,
		hashCOPKey)
	if err != nil {
		return nil, err
	}

	addrToGV, err := freelru.New[libpf.Address, libpf.String](
		interpreter.LruFunctionCacheSize, libpf.Address.Hash32)
	if err != nil {
		return nil, err
	}

	return &perlInstance{
		d:         d,
		rm:        rm,
		bias:      bias,
		addrToHEK: addrToHEK,
		addrToCOP: addrToCOP,
		addrToGV:  addrToGV,
	}, nil
}

func (d *perlData) Unload(_ interpreter.EbpfHandler) {
}

func newData(ebpf interpreter.EbpfHandler, info *interpreter.LoaderInfo,
	ef *pfelf.File) (*perlData, error) {
	// The version is encoded in these globals since Perl 5.15.0.
	// https://github.com/Perl/perl5/blob/v5.32.0/perl.h#L4745-L4754
	var verBytes [3]byte
	for i, sym := range []libpf.SymbolName{"PL_revision", "PL_version", "PL_subversion"} {
		addr, err := ef.LookupSymbolAddress(sym)
		if err == nil {
			_, err = ef.ReadVirtualMemory(verBytes[i:i+1], int64(addr))
		}
		if err != nil {
			return nil, fmt.Errorf("perl symbol '%s': %v", sym, err)
		}
	}

	version := perlVersion(verBytes[0], verBytes[1], verBytes[2])
	log.Debugf("Perl version %v.%v.%v", verBytes[0], verBytes[1], verBytes[2])

	// Currently tested and supported 5.28.x - 5.42.x.
	// Could possibly support older Perl versions somewhere back to 5.14-5.20, by just
	// checking the introspection offset validity. 5.14 had major rework for internals.
	// And 5.18 had some HV related changes.
	minVer := perlVersion(5, 28, 0)
	maxVer := perlVersion(5, 43, 0)
	if version < minVer || version >= maxVer {
		return nil, fmt.Errorf("unsupported Perl %d.%d.%d (need >= %d.%d and < %d.%d)",
			verBytes[0], verBytes[1], verBytes[2],
			(minVer>>16)&0xff, (minVer>>8)&0xff,
			(maxVer>>16)&0xff, (maxVer>>8)&0xff)
	}

	// "PL_thr_key" contains the TSD key since Perl 5.15.2
	// https://github.com/Perl/perl5/blob/v5.32.0/perlvars.h#L45
	stateInTSD := true
	var curcopAddr, cursiAddr libpf.SymbolValue
	stateAddr, err := ef.LookupSymbolAddress("PL_thr_key")
	if err != nil {
		// If Perl is built without threading support, this symbol is not found.
		// Fallback to using the global interpreter state.
		curcopAddr, err = ef.LookupSymbolAddress("PL_curcop")
		if err != nil {
			return nil, fmt.Errorf("perl %x: PL_curcop not found: %v", version, err)
		}
		cursiAddr, err = ef.LookupSymbolAddress("PL_curstackinfo")
		if err != nil {
			return nil, fmt.Errorf("perl %x: PL_curstackinfo not found: %v", version, err)
		}
		stateInTSD = false
		stateAddr = min(curcopAddr, cursiAddr)
	}

	// Perl_runops_standard is the main loop since Perl 5.6.0 (1999)
	// https://github.com/Perl/perl5/blob/v5.32.0/run.c#L37
	// Also Perl_runops_debug exists which is used when the perl debugger is
	// active, but this is not supported currently.
	interpRanges, err := info.GetSymbolAsRanges("Perl_runops_standard")
	if err != nil {
		return nil, err
	}

	d := &perlData{
		version:    version,
		stateAddr:  stateAddr,
		stateInTSD: stateInTSD,
	}

	// Perl does not provide introspection data, hard code the struct field
	// offsets based on detected version. Some values can be fairly easily
	// calculated from the struct definitions, but some are looked up by
	// using gdb and getting the field offset directly from debug data.
	vms := &d.vmStructs
	if stateInTSD {
		if version >= perlVersion(5, 34, 0) {
			// For Perl 5.34 PerlInterpreter changed and so did its offsets.
			vms.interpreter.curcop = 0xd0
			vms.interpreter.curstackinfo = 0xe0
		} else {
			vms.interpreter.curcop = 0xe0
			vms.interpreter.curstackinfo = 0xf0
		}
	} else {
		vms.interpreter.curcop = uint(curcopAddr - stateAddr)
		vms.interpreter.curstackinfo = uint(cursiAddr - stateAddr)
	}
	vms.stackinfo.si_cxstack = 0x08
	vms.stackinfo.si_next = 0x18
	vms.stackinfo.si_cxix = 0x20
	vms.stackinfo.si_type = 0x28
	vms.context.cx_type = 0
	vms.context.blk_oldcop = 0x10
	vms.context.blk_sub_retop = 0x30
	vms.context.blk_sub_cv = 0x40
	vms.context.sizeof = 0x60
	vms.cop.cop_line = 0x24
	vms.cop.cop_file = 0x30
	vms.cop.sizeof = 0x50
	vms.sv.sv_any = 0x0
	vms.sv.sv_flags = 0xc
	vms.sv.svu_gp = 0x10
	vms.sv.svu_hash = 0x10
	vms.sv.sizeof = 0x18
	vms.xpvcv.xcv_flags = 0x5c
	vms.xpvcv.xcv_gv = 0x38
	vms.xpvgv.xivu_namehek = 0x20
	vms.xpvgv.xgv_stash = 0x28
	vms.xpvhv.xhv_max = 0x18
	vms.xpvhv_aux.xhv_name_u = 0x0
	vms.xpvhv_aux.xhv_name_count = 0x1c
	vms.xpvhv_aux.sizeof = 0x38
	vms.xpvhv_aux.pointer_size = 0x8
	vms.gp.gp_egv = 0x38
	vms.hek.hek_len = 0x4
	vms.hek.hek_key = 0x8

	if version >= perlVersion(5, 32, 0) {
		vms.stackinfo.si_type = 0x2c
		vms.context.blk_sub_cv = 0x48
		vms.context.sizeof = 0x68
		vms.cop.sizeof = 0x58
	}

	if version >= perlVersion(5, 35, 0) {
		vms.xpvhv_with_aux.xpvhv_aux = 0x20
	}

	if version >= perlVersion(5, 40, 0) {
		vms.xpvhv_aux.sizeof = 0x70
	}

	if err = ebpf.UpdateInterpreterOffsets(support.ProgUnwindPerl,
		info.FileID(), interpRanges); err != nil {
		return nil, err
	}

	return d, nil
}
