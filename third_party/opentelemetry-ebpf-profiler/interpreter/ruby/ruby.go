// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ruby // import "go.opentelemetry.io/ebpf-profiler/interpreter/ruby"

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math/bits"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"unsafe"

	log "github.com/sirupsen/logrus"

	"github.com/elastic/go-freelru"

	"go.opentelemetry.io/ebpf-profiler/host"
	"go.opentelemetry.io/ebpf-profiler/interpreter"
	"go.opentelemetry.io/ebpf-profiler/libpf"
	"go.opentelemetry.io/ebpf-profiler/libpf/pfelf"
	"go.opentelemetry.io/ebpf-profiler/libpf/pfunsafe"
	"go.opentelemetry.io/ebpf-profiler/metrics"
	"go.opentelemetry.io/ebpf-profiler/remotememory"
	"go.opentelemetry.io/ebpf-profiler/successfailurecounter"
	"go.opentelemetry.io/ebpf-profiler/support"
	"go.opentelemetry.io/ebpf-profiler/util"
)

const (
	// iseqCacheSize is the LRU size for caching Ruby instruction sequences for an interpreter.
	// This should reflect the number of hot functions that are seen often in a trace.
	iseqCacheSize = 1024
	// addrToStringSize is the LRU size for caching Ruby VM addresses to Ruby strings.
	addrToStringSize = 1024

	// rubyInsnInfoSizeLimit defines the limit up to which we will allocate memory for the
	// binary search algorithm to get the line number.
	rubyInsnInfoSizeLimit = 1 * 1024 * 1024
)

//nolint:lll
const (
	// RUBY_T_STRING
	// https://github.com/ruby/ruby/blob/c149708018135595b2c19c5f74baf9475674f394/include/ruby/internal/value_type.h#L117
	rubyTString = 0x5

	// RUBY_T_ARRAY
	// https://github.com/ruby/ruby/blob/c149708018135595b2c19c5f74baf9475674f394/include/ruby/internal/value_type.h#L119
	rubyTArray = 0x7

	// RUBY_T_MASK
	// https://github.com/ruby/ruby/blob/c149708018135595b2c19c5f74baf9475674f394/include/ruby/internal/value_type.h#L142
	rubyTMask = 0x1f

	// RSTRING_NOEMBED
	// https://github.com/ruby/ruby/blob/5445e0435260b449decf2ac16f9d09bae3cafe72/include/ruby/ruby.h#L978
	// https://github.com/ruby/ruby/blob/5445e0435260b449decf2ac16f9d09bae3cafe72/include/ruby/ruby.h#L855
	// 1 << 13
	rstringNoEmbed = 8192

	// RARRAY_EMBED_FLAG
	rarrayEmbed = 8192

	// PATHOBJ_REALPATH
	pathObjRealPathIdx = 1
)

var (
	// regex to identify the Ruby interpreter executable
	rubyRegex = regexp.MustCompile(`^(?:.*/)?libruby(?:-.*)?\.so\.(\d)\.(\d)\.(\d)$`)
	// regex to extract a version from a string
	rubyVersionRegex = regexp.MustCompile(`^(\d)\.(\d)\.(\d)$`)

	// compiler check to make sure the needed interfaces are satisfied
	_ interpreter.Data     = &rubyData{}
	_ interpreter.Instance = &rubyInstance{}
)

//nolint:lll
type rubyData struct {
	// currentCtxPtr is the `ruby_current_execution_context_ptr` symbol value which is needed by the
	// eBPF program to build ruby backtraces.
	currentCtxPtr libpf.Address

	// version of the currently used Ruby interpreter.
	// major*0x10000 + minor*0x100 + release (e.g. 3.0.1 -> 0x30001)
	version uint32

	// vmStructs reflects the Ruby internal names and offsets of named fields.
	vmStructs struct {
		// rb_execution_context_struct
		// https://github.com/ruby/ruby/blob/5445e0435260b449decf2ac16f9d09bae3cafe72/vm_core.h#L843
		execution_context_struct struct {
			vm_stack, vm_stack_size, cfp uint8
		}

		// rb_control_frame_struct
		// https://github.com/ruby/ruby/blob/5445e0435260b449decf2ac16f9d09bae3cafe72/vm_core.h#L760
		control_frame_struct struct {
			pc, iseq, ep                 uint8
			size_of_control_frame_struct uint8
		}

		// rb_iseq_struct
		// https://github.com/ruby/ruby/blob/5445e0435260b449decf2ac16f9d09bae3cafe72/vm_core.h#L456
		iseq_struct struct {
			body uint8
		}

		// rb_iseq_constant_body
		// https://github.com/ruby/ruby/blob/5445e0435260b449decf2ac16f9d09bae3cafe72/vm_core.h#L311
		iseq_constant_body struct {
			iseq_type, encoded, size, location, insn_info_body, insn_info_size, succ_index_table uint8
			size_of_iseq_constant_body                                                           uint16
		}

		// rb_iseq_location_struct
		// https://github.com/ruby/ruby/blob/5445e0435260b449decf2ac16f9d09bae3cafe72/vm_core.h#L272
		iseq_location_struct struct {
			pathobj, base_label uint8
		}

		// succ_index_table_struct
		// https://github.com/ruby/ruby/blob/5445e0435260b449decf2ac16f9d09bae3cafe72/iseq.c#L3420
		succ_index_table_struct struct {
			small_block_ranks, block_bits, succ_part, succ_dict_block uint8
			size_of_succ_dict_block                                   uint8
		}

		// iseq_insn_info_entry
		// https://github.com/ruby/ruby/blob/4e0a512972cdcbfcd5279f1a2a81ba342ed75b6e/iseq.h#L212
		iseq_insn_info_entry struct {
			position, line_no                                               uint8
			size_of_position, size_of_line_no, size_of_iseq_insn_info_entry uint8
		}

		// RString
		// https://github.com/ruby/ruby/blob/5445e0435260b449decf2ac16f9d09bae3cafe72/include/ruby/ruby.h#L988
		// https://github.com/ruby/ruby/blob/86ac17efde6cf98903513cac2538b15fc4ac80b2/include/ruby/internal/core/rstring.h#L196
		rstring_struct struct {
			// NOTE: starting with Ruby 3.1 the `as.ary` field is now `as.embed.ary`
			as_heap_ptr, as_ary uint8
		}

		// RArray
		// https://github.com/ruby/ruby/blob/5445e0435260b449decf2ac16f9d09bae3cafe72/include/ruby/ruby.h#L1048
		rarray_struct struct {
			as_heap_ptr, as_ary uint8
		}

		// size_of_immediate_table holds the size of the macro IMMEDIATE_TABLE_SIZE as defined in
		// https://github.com/ruby/ruby/blob/5445e0435260b449decf2ac16f9d09bae3cafe72/iseq.c#L3418
		size_of_immediate_table uint8

		// size_of_value holds the size of the macro VALUE as defined in
		// https://github.com/ruby/ruby/blob/5445e0435260b449decf2ac16f9d09bae3cafe72/vm_core.h#L1136
		size_of_value uint8

		// rb_ractor_struct
		// https://github.com/ruby/ruby/blob/5ce0d2aa354eb996cb3ca9bb944f880ff6acfd57/ractor_core.h#L82
		rb_ractor_struct struct {
			running_ec uint16
		}
	}
}

func rubyVersion(major, minor, release uint32) uint32 {
	return major*0x10000 + minor*0x100 + release
}

func (r *rubyData) Attach(ebpf interpreter.EbpfHandler, pid libpf.PID, bias libpf.Address,
	rm remotememory.RemoteMemory) (interpreter.Instance, error) {
	cdata := support.RubyProcInfo{
		Version: r.version,

		Current_ctx_ptr: uint64(r.currentCtxPtr + bias),

		Vm_stack:      r.vmStructs.execution_context_struct.vm_stack,
		Vm_stack_size: r.vmStructs.execution_context_struct.vm_stack_size,
		Cfp:           r.vmStructs.execution_context_struct.cfp,

		Pc:                           r.vmStructs.control_frame_struct.pc,
		Iseq:                         r.vmStructs.control_frame_struct.iseq,
		Ep:                           r.vmStructs.control_frame_struct.ep,
		Size_of_control_frame_struct: r.vmStructs.control_frame_struct.size_of_control_frame_struct,

		Body: r.vmStructs.iseq_struct.body,

		Iseq_size:    r.vmStructs.iseq_constant_body.size,
		Iseq_encoded: r.vmStructs.iseq_constant_body.encoded,

		Size_of_value: r.vmStructs.size_of_value,

		Running_ec: r.vmStructs.rb_ractor_struct.running_ec,
	}

	if err := ebpf.UpdateProcData(libpf.Ruby, pid, unsafe.Pointer(&cdata)); err != nil {
		return nil, err
	}

	addrToString, err := freelru.New[libpf.Address, libpf.String](addrToStringSize,
		libpf.Address.Hash32)
	if err != nil {
		return nil, err
	}

	return &rubyInstance{
		r:            r,
		rm:           rm,
		addrToString: addrToString,
		memPool: sync.Pool{
			New: func() any {
				buf := make([]byte, 512)
				return &buf
			},
		},
	}, nil
}

func (r *rubyData) Unload(_ interpreter.EbpfHandler) {
}

type rubyInstance struct {
	interpreter.InstanceStubs

	// Ruby symbolization metrics
	successCount atomic.Uint64
	failCount    atomic.Uint64

	r  *rubyData
	rm remotememory.RemoteMemory

	// addrToString maps an address to an extracted Ruby String from this address.
	addrToString *freelru.LRU[libpf.Address, libpf.String]

	// memPool provides pointers to byte arrays for efficient memory reuse.
	memPool sync.Pool

	// maxSize is the largest number we did see in the last reporting interval for size
	// in getRubyLineNo.
	maxSize atomic.Uint32
}

func (r *rubyInstance) Detach(ebpf interpreter.EbpfHandler, pid libpf.PID) error {
	return ebpf.DeleteProcData(libpf.Ruby, pid)
}

// readRubyArrayDataPtr obtains the data pointer of a Ruby array (RArray).
//
// https://github.com/ruby/ruby/blob/95aff2146/include/ruby/internal/core/rarray.h#L87
func (r *rubyInstance) readRubyArrayDataPtr(addr libpf.Address) (libpf.Address, error) {
	flags := r.rm.Ptr(addr)
	if flags&rubyTMask != rubyTArray {
		return 0, fmt.Errorf("object at 0x%08X is not an array", addr)
	}

	vms := &r.r.vmStructs
	if flags&rarrayEmbed == rarrayEmbed {
		return addr + libpf.Address(vms.rarray_struct.as_ary), nil
	}

	p := r.rm.Ptr(addr + libpf.Address(vms.rarray_struct.as_heap_ptr))
	if p != 0 {
		return 0, fmt.Errorf("heap pointer of array at 0x%08X is 0", addr)
	}

	return addr, nil
}

// readPathObjRealPath reads the realpath field from a Ruby iseq pathobj.
//
// Path objects are represented as either a Ruby string (RString) or a
// Ruby arrays (RArray) with 2 entries. The first field contains a relative
// path, the second one an absolute one. All Ruby types start with an RBasic
// object that contains a type tag that we can use to determine what variant
// we're dealing with.
//
// https://github.com/ruby/ruby/blob/4e0a51297/iseq.c#L217
// https://github.com/ruby/ruby/blob/95aff2146/vm_core.h#L267
// https://github.com/ruby/ruby/blob/95aff2146/vm_core.h#L283
// https://github.com/ruby/ruby/blob/7127f39ba/vm_core.h#L321-L321
func (r *rubyInstance) readPathObjRealPath(addr libpf.Address) (string, error) {
	flags := r.rm.Ptr(addr)
	switch flags & rubyTMask {
	case rubyTString:
		// nothing to do
	case rubyTArray:
		var err error
		addr, err = r.readRubyArrayDataPtr(addr)
		if err != nil {
			return "", err
		}

		addr += pathObjRealPathIdx * libpf.Address(r.r.vmStructs.size_of_value)
		addr = r.rm.Ptr(addr) // deref VALUE -> RString object
	default:
		return "", fmt.Errorf("unexpected pathobj type tag: 0x%X", flags&rubyTMask)
	}

	return r.readRubyString(addr)
}

// readRubyString extracts a Ruby string from the given addr.
//
// 2.5.0: https://github.com/ruby/ruby/blob/4e0a51297/include/ruby/ruby.h#L1004
// 3.0.0: https://github.com/ruby/ruby/blob/48b94b791/include/ruby/internal/core/rstring.h#L73
func (r *rubyInstance) readRubyString(addr libpf.Address) (string, error) {
	flags := r.rm.Ptr(addr)
	if flags&rubyTMask != rubyTString {
		return "", fmt.Errorf("object at 0x%08X is not a string", addr)
	}

	var str string
	vms := &r.r.vmStructs
	if flags&rstringNoEmbed == rstringNoEmbed {
		str = r.rm.StringPtr(addr + libpf.Address(vms.rstring_struct.as_heap_ptr))
	} else {
		str = r.rm.String(addr + libpf.Address(vms.rstring_struct.as_ary))
	}

	r.addrToString.Add(addr, libpf.Intern(str))
	return str, nil
}

type StringReader = func(address libpf.Address) (string, error)

// getStringCached retrieves a string from cache or reads and inserts it if it's missing.
func (r *rubyInstance) getStringCached(addr libpf.Address, reader StringReader) (
	libpf.String, error) {
	if value, ok := r.addrToString.Get(addr); ok {
		return value, nil
	}

	str, err := reader(addr)
	if err != nil {
		return libpf.NullString, err
	}
	if !util.IsValidString(str) {
		log.Debugf("Extracted invalid string from Ruby at 0x%x '%v'[len=%d]",
			addr, unsafe.Slice(unsafe.StringData(str), min(len(str), 128)), len(str))
		return libpf.NullString, fmt.Errorf("extracted invalid Ruby string from address 0x%x", addr)
	}

	val := libpf.Intern(str)
	r.addrToString.Add(addr, val)
	return val, err
}

// rubyPopcount64 is a helper macro.
// Ruby makes use of __builtin_popcount intrinsics. These builtin intrinsics are not available
// here so we use the equivalent function of the Go standard library.
// https://github.com/ruby/ruby/blob/48b94b791997881929c739c64f95ac30f3fd0bb9/internal/bits.h#L408
func rubyPopcount64(in uint64) uint32 {
	return uint32(bits.OnesCount64(in))
}

// smallBlockRankGet is a helper macro.
// https://github.com/ruby/ruby/blob/5445e0435260b449decf2ac16f9d09bae3cafe72/iseq.c#L3432
func smallBlockRankGet(v uint64, i uint32) uint32 {
	if i == 0 {
		return 0
	}
	return uint32((v >> ((i - 1) * 9))) & 0x1ff
}

// immBlockRankGet is a helper macro.
// https://github.com/ruby/ruby/blob/5445e0435260b449decf2ac16f9d09bae3cafe72/iseq.c#L3430
func immBlockRankGet(v uint64, i uint32) uint32 {
	tmp := v >> (i * 7)
	return uint32(tmp) & 0x7f
}

// getObsoleteRubyLineNo implements a binary search algorithm to get the line number for a position.
//
// Implementation according to Ruby:
// https://github.com/ruby/ruby/blob/4e0a512972cdcbfcd5279f1a2a81ba342ed75b6e/iseq.c#L1254-L1295
func (r *rubyInstance) getObsoleteRubyLineNo(iseqBody libpf.Address,
	pos, size uint32) (uint32, error) {
	vms := &r.r.vmStructs
	sizeOfEntry := uint32(vms.iseq_insn_info_entry.size_of_iseq_insn_info_entry)

	ptr := r.rm.Ptr(iseqBody + libpf.Address(vms.iseq_constant_body.insn_info_body))
	syncPoolData := r.memPool.Get().(*[]byte)
	if syncPoolData == nil {
		return 0, errors.New("failed to get memory from sync pool")
	}
	if uint32(len(*syncPoolData)) < size*sizeOfEntry {
		// make sure the data we want to write into blob fits in
		*syncPoolData = make([]byte, size*sizeOfEntry)
	}
	defer func() {
		// Reset memory and return it for reuse.
		for i := uint32(0); i < size*sizeOfEntry; i++ {
			(*syncPoolData)[i] = 0x0
		}
		r.memPool.Put(syncPoolData)
	}()
	blob := (*syncPoolData)[:size*sizeOfEntry]

	// Read the table with multiple iseq_insn_info_entry entries only once for the binary search.
	if err := r.rm.Read(ptr, blob); err != nil {
		return 0, fmt.Errorf("failed to read line table for binary search: %v", err)
	}

	var blobPos uint32
	var entryPos, entryLine uint32
	right := size - 1
	left := uint32(1)

	posOffset := uint32(vms.iseq_insn_info_entry.position)
	posSize := uint32(vms.iseq_insn_info_entry.size_of_position)
	lineNoOffset := uint32(vms.iseq_insn_info_entry.line_no)
	lineNoSize := uint32(vms.iseq_insn_info_entry.size_of_line_no)

	for left <= right {
		index := left + (right-left)/2

		blobPos = index * sizeOfEntry

		entryPos = binary.LittleEndian.Uint32(
			blob[blobPos+posOffset : blobPos+posOffset+posSize])
		entryLine = binary.LittleEndian.Uint32(
			blob[blobPos+lineNoOffset : blobPos+lineNoOffset+lineNoSize])

		if entryPos == pos {
			return entryLine, nil
		}

		if entryPos < pos {
			left = index + 1
			continue
		}
		right = index - 1
	}

	if left >= size {
		blobPos = (size - 1) * sizeOfEntry
		return binary.LittleEndian.Uint32(
			blob[blobPos+lineNoOffset : blobPos+lineNoOffset+lineNoSize]), nil
	}

	blobPos = left * sizeOfEntry
	entryPos = binary.LittleEndian.Uint32(blob[blobPos+posOffset : blobPos+posOffset+posSize])

	if entryPos > pos {
		blobPos = (left - 1) * sizeOfEntry
		return binary.LittleEndian.Uint32(
			blob[blobPos+lineNoOffset : blobPos+lineNoOffset+lineNoSize]), nil
	}
	return binary.LittleEndian.Uint32(
		blob[blobPos+lineNoOffset : blobPos+lineNoOffset+lineNoSize]), nil
}

// getRubyLineNo extracts the line number information from the given instruction sequence body and
// Ruby VM program counter.
// Starting with Ruby version 2.6.0 [0] Ruby no longer stores the information about the line number
// in a struct field but encodes them in a succinct data structure [1].
// For the lookup of the line number in this data structure getRubyLineNo follows the naming and
// implementation of the Ruby internal function succ_index_lookup [2].
//
// [0] https://github.com/ruby/ruby/commit/83262f24896abeaf1977c8837cbefb1b27040bef
// [1] https://en.wikipedia.org/wiki/Succinct_data_structure
// [2] https://github.com/ruby/ruby/blob/5445e0435260b449decf2ac16f9d09bae3cafe72/iseq.c#L3500-L3517
func (r *rubyInstance) getRubyLineNo(iseqBody libpf.Address, pc uint64) (uint32, error) {
	vms := &r.r.vmStructs

	// Read the struct iseq_constant_body only once.
	blob := make([]byte, vms.iseq_constant_body.size_of_iseq_constant_body)
	if err := r.rm.Read(iseqBody, blob); err != nil {
		return 0, fmt.Errorf("failed to read iseq_constant_body: %v", err)
	}

	offsetEncoded := vms.iseq_constant_body.encoded
	iseqEncoded := binary.LittleEndian.Uint64(blob[offsetEncoded : offsetEncoded+8])

	offsetSize := vms.iseq_constant_body.insn_info_size
	size := binary.LittleEndian.Uint32(blob[offsetSize : offsetSize+4])

	// For our better understanding and future improvement we track the maximum value we get for
	// size and report it.
	util.AtomicUpdateMaxUint32(&r.maxSize, size)

	// https://github.com/ruby/ruby/blob/5445e0435260b449decf2ac16f9d09bae3cafe72/iseq.c#L1678
	if size == 0 {
		return 0, errors.New("failed to read size")
	}
	if size == 1 {
		offsetBody := vms.iseq_constant_body.insn_info_body
		lineNo := binary.LittleEndian.Uint32(blob[offsetBody : offsetBody+4])
		return lineNo, nil
	}
	if size > rubyInsnInfoSizeLimit {
		// When reading the value for size we don't have a way to validate this returned
		// value. To make sure we don't accept any arbitrary number we set here a limit of
		// 1MB.
		// Returning 0 here is not the correct line number at this point. But we let the
		// rest of the symbolization process unwind the frame and get the file name. This
		// way we can provide partial results.
		return 0, nil
	}

	// To get the line number iseq_encoded is subtracted from pc. This result also represents the
	// size of the current instruction sequence. If the calculated size of the instruction sequence
	// is greater than the value in iseq_encoded we don't report this pc to user space.
	//
	//nolint:lll
	// https://github.com/ruby/ruby/blob/5445e0435260b449decf2ac16f9d09bae3cafe72/vm_backtrace.c#L47-L48
	pos := (pc - iseqEncoded) / uint64(vms.size_of_value)
	if pos != 0 {
		pos--
	}

	// Ruby 2.6 changed the way of storing line numbers with [0]. As we still want to get
	// the line number information for older Ruby versions, we have this special
	// handling here.
	//
	// [0] https://github.com/ruby/ruby/commit/83262f24896abeaf1977c8837cbefb1b27040bef
	if r.r.version < 0x20600 {
		return r.getObsoleteRubyLineNo(iseqBody, uint32(pos), size)
	}

	offsetSuccTable := vms.iseq_constant_body.succ_index_table
	succIndexTable := binary.LittleEndian.Uint64(blob[offsetSuccTable : offsetSuccTable+8])

	if succIndexTable == 0 {
		// https://github.com/ruby/ruby/blob/5445e0435260b449decf2ac16f9d09bae3cafe72/iseq.c#L1686
		return 0, errors.New("failed to get table with line information")
	}

	// https://github.com/ruby/ruby/blob/5445e0435260b449decf2ac16f9d09bae3cafe72/iseq.c#L3500-L3517
	var tableIndex uint32
	if pos < uint64(vms.size_of_immediate_table) {
		i := int(pos / 9)
		j := int(pos % 9)
		immPart := r.rm.Uint64(libpf.Address(succIndexTable) +
			libpf.Address(i*int(vms.size_of_value)))
		if immPart == 0 {
			return 0, errors.New("failed to read immPart")
		}
		tableIndex = immBlockRankGet(immPart, uint32(j))
	} else {
		blockIndex := uint32((pos - uint64(vms.size_of_immediate_table)) / 512)
		blockOffset := libpf.Address(blockIndex *
			uint32(vms.succ_index_table_struct.size_of_succ_dict_block))

		rank := r.rm.Uint32(libpf.Address(succIndexTable) +
			libpf.Address(vms.succ_index_table_struct.succ_part) + blockOffset)
		if rank == 0 {
			return 0, errors.New("failed to read rank")
		}

		blockBitIndex := uint32((pos - uint64(vms.size_of_immediate_table)) % 512)
		smallBlockIndex := blockBitIndex / 64
		smallBlockOffset := libpf.Address(smallBlockIndex * uint32(vms.size_of_value))

		smallBlockRanks := r.rm.Uint64(libpf.Address(succIndexTable) + blockOffset +
			libpf.Address(vms.succ_index_table_struct.succ_part+
				vms.succ_index_table_struct.small_block_ranks))
		if smallBlockRanks == 0 {
			return 0, errors.New("failed to read smallBlockRanks")
		}

		smallBlockPopcount := smallBlockRankGet(smallBlockRanks, smallBlockIndex)

		blockBits := r.rm.Uint64(libpf.Address(succIndexTable) + blockOffset +
			libpf.Address(vms.succ_index_table_struct.succ_part+
				vms.succ_index_table_struct.block_bits) + smallBlockOffset)
		if blockBits == 0 {
			return 0, errors.New("failed to read blockBits")
		}
		popCnt := rubyPopcount64((blockBits << (63 - blockBitIndex%64)))

		tableIndex = rank + smallBlockPopcount + popCnt
	}
	tableIndex--

	offsetBody := vms.iseq_constant_body.insn_info_body
	lineNoAddr := binary.LittleEndian.Uint64(blob[offsetBody : offsetBody+8])
	if lineNoAddr == 0 {
		return 0, errors.New("failed to read lineNoAddr")
	}

	lineNo := r.rm.Uint32(libpf.Address(lineNoAddr) +
		libpf.Address(tableIndex*uint32(vms.iseq_insn_info_entry.size_of_iseq_insn_info_entry)))
	if lineNo == 0 {
		return 0, errors.New("failed to read lineNo")
	}
	return lineNo, nil
}

func (r *rubyInstance) Symbolize(frame *host.Frame, frames *libpf.Frames) error {
	if !frame.Type.IsInterpType(libpf.Ruby) {
		return interpreter.ErrMismatchInterpreterType
	}
	vms := &r.r.vmStructs

	sfCounter := successfailurecounter.New(&r.successCount, &r.failCount)
	defer sfCounter.DefaultToFailure()

	// From the eBPF Ruby unwinder we receive the address to the instruction sequence body in
	// the Files field.
	//
	// rb_iseq_constant_body
	// https://github.com/ruby/ruby/blob/5445e0435260b449decf2ac16f9d09bae3cafe72/vm_core.h#L311
	iseqBody := libpf.Address(frame.File)

	// The Ruby VM program counter that was extracted from the current call frame is embedded in
	// the Linenos field.
	pc := frame.Lineno

	lineNo, err := r.getRubyLineNo(iseqBody, uint64(pc))
	if err != nil {
		return err
	}

	sourceFileNamePtr := r.rm.Ptr(iseqBody +
		libpf.Address(vms.iseq_constant_body.location+vms.iseq_location_struct.pathobj))
	sourceFileName, err := r.getStringCached(sourceFileNamePtr, r.readPathObjRealPath)
	if err != nil {
		return err
	}

	funcNamePtr := r.rm.Ptr(iseqBody +
		libpf.Address(vms.iseq_constant_body.location+vms.iseq_location_struct.base_label))
	functionName, err := r.getStringCached(funcNamePtr, r.readRubyString)
	if err != nil {
		return err
	}

	// Ruby doesn't provide the information about the function offset for the
	// particular line. So we report 0 for this to our backend.
	frames.Append(&libpf.Frame{
		Type:         libpf.RubyFrame,
		FunctionName: functionName,
		SourceFile:   sourceFileName,
		SourceLine:   libpf.SourceLineno(lineNo),
	})
	sfCounter.ReportSuccess()
	return nil
}

func (r *rubyInstance) GetAndResetMetrics() ([]metrics.Metric, error) {
	addrToStringStats := r.addrToString.ResetMetrics()

	return []metrics.Metric{
		{
			ID:    metrics.IDRubySymbolizationSuccess,
			Value: metrics.MetricValue(r.successCount.Swap(0)),
		},
		{
			ID:    metrics.IDRubySymbolizationFailure,
			Value: metrics.MetricValue(r.failCount.Swap(0)),
		},
		{
			ID:    metrics.IDRubyAddrToStringHit,
			Value: metrics.MetricValue(addrToStringStats.Hits),
		},
		{
			ID:    metrics.IDRubyAddrToStringMiss,
			Value: metrics.MetricValue(addrToStringStats.Misses),
		},
		{
			ID:    metrics.IDRubyAddrToStringAdd,
			Value: metrics.MetricValue(addrToStringStats.Inserts),
		},
		{
			ID:    metrics.IDRubyAddrToStringDel,
			Value: metrics.MetricValue(addrToStringStats.Removals),
		},
		{
			ID:    metrics.IDRubyMaxSize,
			Value: metrics.MetricValue(r.maxSize.Swap(0)),
		},
	}, nil
}

// determineRubyVersion looks for the symbol ruby_version and extracts version
// information from its value.
func determineRubyVersion(ef *pfelf.File) (uint32, error) {
	_, memory, err := ef.SymbolData("ruby_version", 64)
	if err != nil {
		return 0, fmt.Errorf("unable to read 'ruby_version': %v", err)
	}

	versionString := strings.TrimRight(pfunsafe.ToString(memory), "\x00")
	matches := rubyVersionRegex.FindStringSubmatch(versionString)
	if len(matches) < 3 {
		return 0, fmt.Errorf("failed to parse version string: '%s'", versionString)
	}
	major, _ := strconv.Atoi(matches[1])
	minor, _ := strconv.Atoi(matches[2])
	release, _ := strconv.Atoi(matches[3])

	return rubyVersion(uint32(major), uint32(minor), uint32(release)), nil
}

func Loader(ebpf interpreter.EbpfHandler, info *interpreter.LoaderInfo) (interpreter.Data, error) {
	if !rubyRegex.MatchString(info.FileName()) {
		return nil, nil
	}

	ef, err := info.GetELF()
	if err != nil {
		return nil, err
	}

	version, err := determineRubyVersion(ef)
	if err != nil {
		return nil, err
	}

	// Reason for lowest supported version:
	// - Ruby 2.5 is still commonly used at time of writing this code.
	//   https://www.jetbrains.com/lp/devecosystem-2020/ruby/
	// Reason for maximum supported version 3.2.x:
	// - this is currently the newest stable version

	minVer, maxVer := rubyVersion(2, 5, 0), rubyVersion(3, 3, 0)
	if version < minVer || version >= maxVer {
		return nil, fmt.Errorf("unsupported Ruby %d.%d.%d (need >= %d.%d.%d and <= %d.%d.%d)",
			(version>>16)&0xff, (version>>8)&0xff, version&0xff,
			(minVer>>16)&0xff, (minVer>>8)&0xff, minVer&0xff,
			(maxVer>>16)&0xff, (maxVer>>8)&0xff, maxVer&0xff)
	}

	// Before Ruby 2.5 the symbol ruby_current_thread was used for the current execution
	// context but got replaced in [0] with ruby_current_execution_context_ptr.
	// With [1] the Ruby internal execution model changed and the symbol
	// ruby_current_execution_context_ptr was removed. Therefore we need to lookup different
	// symbols depending on the version.
	// [0] https://github.com/ruby/ruby/commit/837fd5e494731d7d44786f29e7d6e8c27029806f
	// [1] https://github.com/ruby/ruby/commit/79df14c04b452411b9d17e26a398e491bca1a811
	currentCtxSymbol := libpf.SymbolName("ruby_single_main_ractor")
	if version < rubyVersion(3, 0, 0) {
		currentCtxSymbol = "ruby_current_execution_context_ptr"
	}
	currentCtxPtr, err := ef.LookupSymbolAddress(currentCtxSymbol)
	if err != nil {
		return nil, fmt.Errorf("%v not found: %v", currentCtxSymbol, err)
	}

	// rb_vm_exec is used to execute the Ruby frames in the Ruby VM and is called within
	// ruby_run_node  which is the main executor function since Ruby v1.9.0
	// https://github.com/ruby/ruby/blob/587e6800086764a1b7c959976acef33e230dccc2/main.c#L47
	symbolName := libpf.SymbolName("rb_vm_exec")
	if version < rubyVersion(2, 6, 0) {
		symbolName = libpf.SymbolName("ruby_exec_node")
	}
	interpRanges, err := info.GetSymbolAsRanges(symbolName)
	if err != nil {
		return nil, err
	}

	rid := &rubyData{
		version:       version,
		currentCtxPtr: libpf.Address(currentCtxPtr),
	}

	vms := &rid.vmStructs

	// Ruby does not provide introspection data, hard code the struct field offsets. Some
	// values can be fairly easily calculated from the struct definitions, but some are
	// looked up by using gdb and getting the field offset directly from debug data.
	vms.execution_context_struct.vm_stack = 0
	vms.execution_context_struct.vm_stack_size = 8
	vms.execution_context_struct.cfp = 16

	vms.control_frame_struct.pc = 0
	vms.control_frame_struct.iseq = 16
	vms.control_frame_struct.ep = 32
	switch {
	case version < rubyVersion(2, 6, 0):
		vms.control_frame_struct.size_of_control_frame_struct = 48
	case version < rubyVersion(3, 1, 0):
		// With Ruby 2.6 the field bp was added to rb_control_frame_t
		// https://github.com/ruby/ruby/commit/ed935aa5be0e5e6b8d53c3e7d76a9ce395dfa18b
		vms.control_frame_struct.size_of_control_frame_struct = 56
	default:
		// 3.1 adds new jit_return field at the end.
		// https://github.com/ruby/ruby/commit/9d8cc01b758f9385bd4c806f3daff9719e07faa0
		vms.control_frame_struct.size_of_control_frame_struct = 64
	}
	vms.iseq_struct.body = 16

	vms.iseq_constant_body.iseq_type = 0
	vms.iseq_constant_body.size = 4
	vms.iseq_constant_body.encoded = 8
	vms.iseq_constant_body.location = 64
	switch {
	case version < rubyVersion(2, 6, 0):
		vms.iseq_constant_body.insn_info_body = 112
		vms.iseq_constant_body.insn_info_size = 200
		vms.iseq_constant_body.succ_index_table = 144
		vms.iseq_constant_body.size_of_iseq_constant_body = 288
	case version < rubyVersion(3, 2, 0):
		vms.iseq_constant_body.insn_info_body = 120
		vms.iseq_constant_body.insn_info_size = 136
		vms.iseq_constant_body.succ_index_table = 144
		vms.iseq_constant_body.size_of_iseq_constant_body = 312
	default:
		vms.iseq_constant_body.insn_info_body = 112
		vms.iseq_constant_body.insn_info_size = 128
		vms.iseq_constant_body.succ_index_table = 136
		vms.iseq_constant_body.size_of_iseq_constant_body = 320
	}
	vms.iseq_location_struct.pathobj = 0
	vms.iseq_location_struct.base_label = 8

	switch {
	case version < rubyVersion(2, 6, 0):
		vms.iseq_insn_info_entry.position = 0
		vms.iseq_insn_info_entry.size_of_position = 4
		vms.iseq_insn_info_entry.line_no = 4
		vms.iseq_insn_info_entry.size_of_line_no = 4
		vms.iseq_insn_info_entry.size_of_iseq_insn_info_entry = 12
	case version < rubyVersion(3, 1, 0):
		// The position field was removed from this struct with
		// https://github.com/ruby/ruby/commit/295838e6eb1d063c64f7cde5bbbd13c7768908fd
		vms.iseq_insn_info_entry.position = 0
		vms.iseq_insn_info_entry.size_of_position = 0
		vms.iseq_insn_info_entry.line_no = 0
		vms.iseq_insn_info_entry.size_of_line_no = 4
		vms.iseq_insn_info_entry.size_of_iseq_insn_info_entry = 8
	default:
		// https://github.com/ruby/ruby/commit/0a36cab1b53646062026c3181117fad73802baf4
		vms.iseq_insn_info_entry.position = 0
		vms.iseq_insn_info_entry.size_of_position = 0
		vms.iseq_insn_info_entry.line_no = 0
		vms.iseq_insn_info_entry.size_of_line_no = 4
		vms.iseq_insn_info_entry.size_of_iseq_insn_info_entry = 12
	}
	if version < rubyVersion(3, 2, 0) {
		vms.rstring_struct.as_ary = 16
	} else {
		vms.rstring_struct.as_ary = 24
	}
	vms.rstring_struct.as_heap_ptr = 24

	vms.rarray_struct.as_ary = 16
	vms.rarray_struct.as_heap_ptr = 32

	vms.succ_index_table_struct.small_block_ranks = 8
	vms.succ_index_table_struct.block_bits = 16
	vms.succ_index_table_struct.succ_part = 48
	vms.succ_index_table_struct.size_of_succ_dict_block = 80
	vms.size_of_immediate_table = 54

	vms.size_of_value = 8

	if version >= rubyVersion(3, 0, 0) {
		if runtime.GOARCH == "amd64" {
			vms.rb_ractor_struct.running_ec = 0x208
		} else {
			vms.rb_ractor_struct.running_ec = 0x218
		}
	}

	if err = ebpf.UpdateInterpreterOffsets(support.ProgUnwindRuby, info.FileID(),
		interpRanges); err != nil {
		return nil, err
	}

	return rid, nil
}
