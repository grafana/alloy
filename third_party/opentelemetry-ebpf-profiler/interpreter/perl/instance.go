// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package perl // import "go.opentelemetry.io/ebpf-profiler/interpreter/perl"

import (
	"errors"
	"fmt"
	"sync/atomic"
	"unsafe"

	"github.com/elastic/go-freelru"
	log "github.com/sirupsen/logrus"
	"github.com/zeebo/xxh3"

	"go.opentelemetry.io/ebpf-profiler/host"
	"go.opentelemetry.io/ebpf-profiler/interpreter"
	"go.opentelemetry.io/ebpf-profiler/libpf"
	"go.opentelemetry.io/ebpf-profiler/libpf/pfunsafe"
	"go.opentelemetry.io/ebpf-profiler/metrics"
	npsr "go.opentelemetry.io/ebpf-profiler/nopanicslicereader"
	"go.opentelemetry.io/ebpf-profiler/remotememory"
	"go.opentelemetry.io/ebpf-profiler/successfailurecounter"
	"go.opentelemetry.io/ebpf-profiler/support"
	"go.opentelemetry.io/ebpf-profiler/tpbase"
	"go.opentelemetry.io/ebpf-profiler/util"
)

type perlInstance struct {
	interpreter.InstanceStubs

	// Symbolization metrics
	successCount atomic.Uint64
	failCount    atomic.Uint64

	d    *perlData
	rm   remotememory.RemoteMemory
	bias libpf.Address

	// addrToHEK maps a PERL Hash Element Key (string with hash) to a Go string
	addrToHEK *freelru.LRU[libpf.Address, libpf.String]

	// addrToCOP maps a PERL Control OP (COP) structure to a perlCOP which caches data from it
	addrToCOP *freelru.LRU[copKey, *perlCOP]

	// addrToGV maps a PERL Glob Value (GV) aka "symbol" to its name string
	addrToGV *freelru.LRU[libpf.Address, libpf.String]

	// hekLen is the largest number we did see in the last reporting interval for hekLen
	// in getHEK.
	hekLen atomic.Uint32

	// procInfoInserted tracks whether we've already inserted process info into BPF maps.
	procInfoInserted bool
}

// perlCOP contains information about Perl Control OPS structure
type perlCOP struct {
	sourceFileName libpf.String
	line           libpf.AddressOrLineno
}

// copKey is used as cache key for Perl Control OPS structures.
type copKey struct {
	copAddr  libpf.Address
	funcName libpf.String
}

// hashCopKey returns a 32 bits hash of the input.
// It's main purpose is to hash keys for caching perlCOP values.
func hashCOPKey(k copKey) uint32 {
	h := k.copAddr.Hash()
	return uint32(h ^ xxh3.HashString128(k.funcName.String()).Lo)
}

func (i *perlInstance) UpdateTSDInfo(ebpf interpreter.EbpfHandler, pid libpf.PID,
	tsdInfo tpbase.TSDInfo) error {
	d := i.d
	stateInTSD := uint8(0)
	if d.stateInTSD {
		stateInTSD = 1
	}
	vms := &d.vmStructs
	data := support.PerlProcInfo{
		Version:    d.version,
		StateAddr:  uint64(d.stateAddr) + uint64(i.bias),
		StateInTSD: stateInTSD,

		TsdInfo: support.TSDInfo{
			Offset:     tsdInfo.Offset,
			Multiplier: tsdInfo.Multiplier,
			Indirect:   tsdInfo.Indirect,
		},

		Interpreter_curcop:       uint16(vms.interpreter.curcop),
		Interpreter_curstackinfo: uint16(vms.interpreter.curstackinfo),

		Si_cxstack: uint8(vms.stackinfo.si_cxstack),
		Si_next:    uint8(vms.stackinfo.si_next),
		Si_cxix:    uint8(vms.stackinfo.si_cxix),
		Si_type:    uint8(vms.stackinfo.si_type),

		Context_type:          uint8(vms.context.cx_type),
		Context_blk_oldcop:    uint8(vms.context.blk_oldcop),
		Context_blk_sub_retop: uint8(vms.context.blk_sub_retop),
		Context_blk_sub_cv:    uint8(vms.context.blk_sub_cv),
		Context_sizeof:        uint8(vms.context.sizeof),

		Sv_flags:  uint8(vms.sv.sv_flags),
		Sv_any:    uint8(vms.sv.sv_any),
		Svu_gp:    uint8(vms.sv.svu_gp),
		Xcv_flags: uint8(vms.xpvcv.xcv_flags),
		Xcv_gv:    uint8(vms.xpvcv.xcv_gv),
		Gp_egv:    uint8(vms.gp.gp_egv),
	}

	err := ebpf.UpdateProcData(libpf.Perl, pid, unsafe.Pointer(&data))
	if err != nil {
		return err
	}

	i.procInfoInserted = true
	return nil
}

func (i *perlInstance) Detach(ebpf interpreter.EbpfHandler, pid libpf.PID) error {
	if !i.procInfoInserted {
		return nil
	}
	return ebpf.DeleteProcData(libpf.Perl, pid)
}

func (i *perlInstance) GetAndResetMetrics() ([]metrics.Metric, error) {
	addrToHEKStats := i.addrToHEK.ResetMetrics()
	addrToCOPStats := i.addrToCOP.ResetMetrics()
	addrToGVStats := i.addrToGV.ResetMetrics()

	return []metrics.Metric{
		{
			ID:    metrics.IDPerlSymbolizationSuccess,
			Value: metrics.MetricValue(i.successCount.Swap(0)),
		},
		{
			ID:    metrics.IDPerlSymbolizationFailure,
			Value: metrics.MetricValue(i.failCount.Swap(0)),
		},
		{
			ID:    metrics.IDPerlAddrToHEKHit,
			Value: metrics.MetricValue(addrToHEKStats.Hits),
		},
		{
			ID:    metrics.IDPerlAddrToHEKMiss,
			Value: metrics.MetricValue(addrToHEKStats.Misses),
		},
		{
			ID:    metrics.IDPerlAddrToHEKAdd,
			Value: metrics.MetricValue(addrToHEKStats.Inserts),
		},
		{
			ID:    metrics.IDPerlAddrToHEKDel,
			Value: metrics.MetricValue(addrToHEKStats.Removals),
		},
		{
			ID:    metrics.IDPerlAddrToCOPHit,
			Value: metrics.MetricValue(addrToCOPStats.Hits),
		},
		{
			ID:    metrics.IDPerlAddrToCOPMiss,
			Value: metrics.MetricValue(addrToCOPStats.Misses),
		},
		{
			ID:    metrics.IDPerlAddrToCOPAdd,
			Value: metrics.MetricValue(addrToCOPStats.Inserts),
		},
		{
			ID:    metrics.IDPerlAddrToCOPDel,
			Value: metrics.MetricValue(addrToCOPStats.Removals),
		},
		{
			ID:    metrics.IDPerlAddrToGVHit,
			Value: metrics.MetricValue(addrToGVStats.Hits),
		},
		{
			ID:    metrics.IDPerlAddrToGVMiss,
			Value: metrics.MetricValue(addrToGVStats.Misses),
		},
		{
			ID:    metrics.IDPerlAddrToGVAdd,
			Value: metrics.MetricValue(addrToGVStats.Inserts),
		},
		{
			ID:    metrics.IDPerlAddrToGVDel,
			Value: metrics.MetricValue(addrToGVStats.Removals),
		},
		{
			ID:    metrics.IDPerlHekLen,
			Value: metrics.MetricValue(i.hekLen.Swap(0)),
		},
	}, nil
}

func (i *perlInstance) getHEK(addr libpf.Address) (libpf.String, error) {
	if addr == 0 {
		return libpf.NullString, errors.New("null hek pointer")
	}
	if value, ok := i.addrToHEK.Get(addr); ok {
		return value, nil
	}
	vms := &i.d.vmStructs

	var buf [hekLenLimit]byte

	// Read the Hash Element Key (HEK) length and readahead bytes in
	// attempt to avoid second system call to read the target string.
	// 128 is chosen arbitrarily as "hopefully good enough"; this value can
	// be increased if it turns out to be necessary.
	const hekInitialRead = 128
	if err := i.rm.Read(addr, buf[:hekInitialRead]); err != nil {
		return libpf.NullString, err
	}
	hekLen := npsr.Uint32(buf[:], vms.hek.hek_len)

	// For our better understanding and future improvement we track the maximum value we get for
	// hekLen and report it.
	util.AtomicUpdateMaxUint32(&i.hekLen, hekLen)

	if hekSize := uint32(vms.hek.hek_key) + hekLen; hekSize > hekInitialRead {
		if hekSize > hekLenLimit {
			return libpf.NullString, fmt.Errorf("hek too large (%d)", hekLen)
		}
		if err := i.rm.Read(addr+hekInitialRead, buf[hekInitialRead:]); err != nil {
			return libpf.NullString, err
		}
	}

	s := pfunsafe.ToString(buf[vms.hek.hek_key : vms.hek.hek_key+uint(hekLen)])
	if !util.IsValidString(s) {
		log.Debugf("Extracted invalid hek string at 0x%x '%v'", addr, []byte(s))
		return libpf.NullString, fmt.Errorf("extracted invalid hek string at 0x%x", addr)
	}
	value := libpf.Intern(s)
	i.addrToHEK.Add(addr, value)

	return value, nil
}

func (i *perlInstance) getHVName(hvAddr libpf.Address) (libpf.String, error) {
	if hvAddr == 0 {
		return libpf.NullString, nil
	}
	vms := &i.d.vmStructs
	hv := make([]byte, vms.sv.sizeof)
	if err := i.rm.Read(hvAddr, hv); err != nil {
		return libpf.NullString, err
	}
	hvFlags := npsr.Uint32(hv, vms.sv.sv_flags)
	if hvFlags&SVt_MASK != SVt_PVHV {
		return libpf.NullString, errors.New("not a HV")
	}

	xpvhvAddr := npsr.Ptr(hv, vms.sv.sv_any)
	xpvhvAux := make([]byte, vms.xpvhv_aux.sizeof)
	if i.d.version < perlVersion(5, 35, 0) {
		// The aux structure is at the end of the array. Calculate its address.
		arrayAddr := npsr.Ptr(hv, vms.sv.svu_hash)
		end := i.rm.Uint64(xpvhvAddr + libpf.Address(vms.xpvhv.xhv_max))
		xpvhvAuxAddr := arrayAddr + libpf.Address((end+1)*uint64(vms.xpvhv_aux.pointer_size))
		if err := i.rm.Read(xpvhvAuxAddr, xpvhvAux); err != nil {
			return libpf.NullString, err
		}
	} else {
		// In Perl 5.36.x.XPVHV got replaced with xpvhv_with_aux to hold this information.
		// https://github.com/Perl/perl5/commit/94ee6ed79dbca73d0345b745534477e4017fb990
		if err := i.rm.Read(xpvhvAddr+libpf.Address(vms.xpvhv_with_aux.xpvhv_aux),
			xpvhvAux); err != nil {
			return libpf.NullString, err
		}
	}

	nameCount := npsr.Int32(xpvhvAux, vms.xpvhv_aux.xhv_name_count)
	hekAddr := npsr.Ptr(xpvhvAux, vms.xpvhv_aux.xhv_name_u)
	// A non-zero name count here implies that the
	// GV belongs to a symbol table that has been
	// altered in some way (Perl calls this a Stash, see
	// https://www.perlmonks.org/?node=perlguts#Stashes_and_Globs for more).
	//
	// Stashes can be manipulated directly from Perl code, but it
	// can also happen during normal operation and it messes with the layout of HVs.
	// The exact link for this behavior is here:
	// https://github.com/Perl/perl5/blob/v5.32.0/hv.h#L114
	if nameCount > 0 {
		// When xhv_name_count > 0, it points to a HEK** array and the
		// first element is the name.
		hekAddr = i.rm.Ptr(hekAddr)
	} else if nameCount < 0 {
		// When xhv_name_count < 0, it points to a HEK** array and the
		// second element is the name.
		hekAddr = i.rm.Ptr(hekAddr + libpf.Address(vms.xpvhv_aux.pointer_size))
	}

	return i.getHEK(hekAddr)
}

func (i *perlInstance) getGV(gvAddr libpf.Address, nameOnly bool) (libpf.String, error) {
	if gvAddr == 0 {
		return libpf.NullString, nil
	}
	if value, ok := i.addrToGV.Get(gvAddr); ok {
		return value, nil
	}

	vms := &i.d.vmStructs

	// Follow the GV's "body" pointer to get the function name
	xpvgvAddr := i.rm.Ptr(gvAddr + libpf.Address(vms.sv.sv_any))
	hekAddr := i.rm.Ptr(xpvgvAddr + libpf.Address(vms.xpvgv.xivu_namehek))
	value, err := i.getHEK(hekAddr)
	if err != nil {
		return libpf.NullString, err
	}

	if !nameOnly && value != libpf.NullString {
		stashAddr := i.rm.Ptr(xpvgvAddr + libpf.Address(vms.xpvgv.xgv_stash))
		packageName, err := i.getHVName(stashAddr)
		if err != nil {
			return libpf.NullString, err
		}

		// Build the qualified name
		if packageName == libpf.NullString {
			// per Perl_gv_fullname4
			value = libpf.Intern("__ANON__::" + value.String())
		} else {
			value = libpf.Intern(packageName.String() + "::" + value.String())
		}
	}

	i.addrToGV.Add(gvAddr, value)
	return value, nil
}

// getCOP reads and caches a Control OP from remote interpreter.
// On success, the COP is returned. On error, the error.
func (i *perlInstance) getCOP(copAddr libpf.Address, funcName libpf.String) (
	*perlCOP, error) {
	key := copKey{
		copAddr:  copAddr,
		funcName: funcName,
	}
	if value, ok := i.addrToCOP.Get(key); ok {
		return value, nil
	}

	vms := &i.d.vmStructs
	cop := make([]byte, vms.cop.sizeof)
	if err := i.rm.Read(copAddr, cop); err != nil {
		return nil, err
	}

	var sourceFileName string
	if i.d.stateInTSD {
		// cop_file is a pointer to nul terminated string
		sourceFileAddr := npsr.Ptr(cop, vms.cop.cop_file)
		sourceFileName = i.rm.String(sourceFileAddr)
	} else {
		// cop_file is a pointer to GV
		sourceFileGVAddr := npsr.Ptr(cop, vms.cop.cop_file)
		gvName, err := i.getGV(sourceFileGVAddr, true)
		if err == nil && len(gvName.String()) <= 2 {
			err = fmt.Errorf("sourcefile gv length too small (%d)", len(gvName.String()))
		}
		if err != nil {
			return nil, err
		}
		sourceFileName = gvName.String()[2:]
	}
	if !util.IsValidString(sourceFileName) {
		log.Debugf("Extracted invalid source file name '%v'", []byte(sourceFileName))
		return nil, errors.New("extracted invalid source file name")
	}

	line := npsr.Uint32(cop, vms.cop.cop_line)

	c := &perlCOP{
		sourceFileName: libpf.Intern(sourceFileName),
		line:           libpf.AddressOrLineno(line),
	}
	i.addrToCOP.Add(key, c)
	return c, nil
}

func (i *perlInstance) Symbolize(frame *host.Frame, frames *libpf.Frames) error {
	if !frame.Type.IsInterpType(libpf.Perl) {
		return interpreter.ErrMismatchInterpreterType
	}

	sfCounter := successfailurecounter.New(&i.successCount, &i.failCount)
	defer sfCounter.DefaultToFailure()

	functionName := interpreter.TopLevelFunctionName
	if gvAddr := libpf.Address(frame.File); gvAddr != 0 {
		var err error
		if functionName, err = i.getGV(gvAddr, false); err != nil {
			return fmt.Errorf("failed to get Perl GV %x: %v", gvAddr, err)
		}
	}
	copAddr := libpf.Address(frame.Lineno)
	cop, err := i.getCOP(copAddr, functionName)
	if err != nil {
		return fmt.Errorf("failed to get Perl COP %x: %v", copAddr, err)
	}

	// Since the COP contains all the data without extra work, just always
	// send the symbolization information.
	frames.Append(&libpf.Frame{
		Type:         libpf.PerlFrame,
		FunctionName: functionName,
		SourceFile:   cop.sourceFileName,
		SourceLine:   libpf.SourceLineno(cop.line),
	})
	sfCounter.ReportSuccess()
	return nil
}
