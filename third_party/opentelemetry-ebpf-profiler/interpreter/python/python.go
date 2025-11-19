// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package python // import "go.opentelemetry.io/ebpf-profiler/interpreter/python"

import (
	"bytes"
	"debug/elf"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"reflect"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
	"unsafe"

	log "github.com/sirupsen/logrus"
	"go.opentelemetry.io/ebpf-profiler/asm/amd"
	"go.opentelemetry.io/ebpf-profiler/nativeunwind/elfunwindinfo"

	"github.com/elastic/go-freelru"

	"go.opentelemetry.io/ebpf-profiler/host"
	"go.opentelemetry.io/ebpf-profiler/interpreter"
	"go.opentelemetry.io/ebpf-profiler/libpf"
	"go.opentelemetry.io/ebpf-profiler/libpf/pfelf"
	"go.opentelemetry.io/ebpf-profiler/metrics"
	npsr "go.opentelemetry.io/ebpf-profiler/nopanicslicereader"
	"go.opentelemetry.io/ebpf-profiler/remotememory"
	"go.opentelemetry.io/ebpf-profiler/successfailurecounter"
	"go.opentelemetry.io/ebpf-profiler/support"
	"go.opentelemetry.io/ebpf-profiler/tpbase"
	"go.opentelemetry.io/ebpf-profiler/util"
)

// The following regexs are intended to match either a path to a Python binary or
// library.
var (
	pythonRegex    = regexp.MustCompile(`^(?:.*/)?python(\d)\.(\d+)(d|m|dm)?$`)
	libpythonRegex = regexp.MustCompile(`^(?:.*/)?libpython(\d)\.(\d+)[^/]*`)
)

// pythonVer builds a version number from readable numbers
func pythonVer(major, minor int) uint16 {
	return uint16(major)*0x100 + uint16(minor)
}

//nolint:lll
type pythonData struct {
	version uint16

	autoTLSKey libpf.SymbolValue

	noneStruct libpf.SymbolValue

	// vmStructs reflects the Python Interpreter introspection data we want
	// need to extract data from the runtime. The fields are named as they are
	// in the Python code. Eventually some of these fields will be read from
	// the Python introspection data, and matched using the reflection names.
	vmStructs struct {
		// https://github.com/python/cpython/blob/deaf509e8fc6e0363bd6f26d52ad42f976ec42f2/Include/cpython/object.h#L148
		PyTypeObject struct {
			BasicSize libpf.Address `name:"tp_basicsize"`
			Members   libpf.Address `name:"tp_members"`
		}
		// https://github.com/python/cpython/blob/deaf509e8fc6e0363bd6f26d52ad42f976ec42f2/Include/structmember.h#L18
		PyMemberDef struct {
			Sizeof libpf.Address
			Name   uint `name:"name"`
			Offset uint `name:"offset"`
		}
		// https://github.com/python/cpython/blob/deaf509e8fc6e0363bd6f26d52ad42f976ec42f2/Include/cpython/unicodeobject.h#L72
		PyASCIIObject struct {
			Data uint `name:"data"`
		}
		PyCodeObject struct {
			Sizeof         uint
			ArgCount       uint `name:"co_argcount"`
			KwOnlyArgCount uint `name:"co_kwonlyargcount"`
			Flags          uint `name:"co_flags"`
			FirstLineno    uint `name:"co_firstlineno"`
			Filename       uint `name:"co_filename"`
			Name           uint `name:"co_name"`
			Lnotab         uint `name:"co_lnotab"`
			Linetable      uint `name:"co_linetable"` // Python 3.10+
			QualName       uint `name:"co_qualname"`  // Python 3.11+
		}
		// https://github.com/python/cpython/blob/deaf509e8fc6e0363bd6f26d52ad42f976ec42f2/Include/object.h#L109
		PyVarObject struct {
			ObSize uint `name:"ob_size"`
		}
		PyBytesObject struct {
			Sizeof uint
		}
		// https://github.com/python/cpython/blob/deaf509e8fc6e0363bd6f26d52ad42f976ec42f2/Include/cpython/pystate.h#L82
		PyThreadState struct {
			Frame uint `name:"frame"`
		}
		PyFrameObject struct {
			Back        uint `name:"f_back"`
			Code        uint `name:"f_code"`
			LastI       uint `name:"f_lasti"`
			EntryMember uint // field depends on python version
			EntryVal    uint // value depends on python version
		}
		// https://github.com/python/cpython/blob/deaf509e8fc6e0363bd6f26d52ad42f976ec42f2/Include/cpython/pystate.h#L38
		PyCFrame struct {
			CurrentFrame uint `name:"current_frame"`
		}
	}
}

var _ interpreter.Data = &pythonData{}

func (d *pythonData) String() string {
	return fmt.Sprintf("Python %d.%d", d.version>>8, d.version&0xff)
}

func (d *pythonData) Attach(_ interpreter.EbpfHandler, _ libpf.PID, bias libpf.Address,
	rm remotememory.RemoteMemory) (interpreter.Instance, error) {
	addrToCodeObject, err :=
		freelru.New[libpf.Address, *pythonCodeObject](interpreter.LruFunctionCacheSize,
			libpf.Address.Hash32)
	if err != nil {
		return nil, err
	}

	i := &pythonInstance{
		d:                d,
		rm:               rm,
		bias:             bias,
		addrToCodeObject: addrToCodeObject,
	}

	switch {
	case d.version >= pythonVer(3, 11):
		i.getFuncOffset = walkLocationTable
	case d.version == pythonVer(3, 10):
		i.getFuncOffset = walkLineTable
	default:
		i.getFuncOffset = mapByteCodeIndexToLine
	}

	return i, nil
}

func (d *pythonData) Unload(_ interpreter.EbpfHandler) {
}

// pythonCodeObject contains the information we cache for a corresponding
// Python interpreter's PyCodeObject structures.
type pythonCodeObject struct {
	// As of Python 3.10 elements of PyCodeObject have changed and so we need
	// to handle them differently. To be able to do so we keep track of the python version.
	version uint16

	// name is the extracted co_name (the unqualified method or function name)
	name libpf.String

	// sourceFileName is the extracted co_filename field
	sourceFileName libpf.String

	// For Python version < 3.10 lineTable is the extracted co_lnotab, and contains the
	// "bytecode index" to "line number" mapping data.
	// For Python version >= 3.10 lineTable is the extracted co_linetable.
	lineTable []byte

	// firstLineNo is the extracted co_firstlineno field, and contains the line
	// number where the method definition in source code starts
	firstLineNo uint32

	// ebpfChecksum is the simple hash of few PyCodeObject fields sent from eBPF
	// to verify that the data we extracted from remote process is still valid
	ebpfChecksum uint32
}

// readVarint returns a variable length encoded unsigned integer from a location table entry.
func readVarint(r io.ByteReader) uint32 {
	val := uint32(0)
	b := byte(0x40)
	for shift := 0; b&0x40 != 0; shift += 6 {
		var err error
		b, err = r.ReadByte()
		if err != nil || b&0x80 != 0 {
			return 0
		}
		val |= uint32(b&0x3f) << shift
	}
	return val
}

// readSignedVarint returns a variable length encoded signed integer from a location table entry.
func readSignedVarint(r io.ByteReader) int32 {
	uval := readVarint(r)
	if uval&1 != 0 {
		return -int32(uval >> 1)
	}
	return int32(uval >> 1)
}

// walkLocationTable implements the algorithm to read entries from the location table.
// This was introduced in Python 3.11.
// https://github.com/python/cpython/blob/deaf509e8fc6e0363bd6f26d52ad42f976ec42f2/Objects/locations.md
//
//nolint:lll
func walkLocationTable(m *pythonCodeObject, bci uint32) uint32 {
	r := bytes.NewReader(m.lineTable)
	curI := uint32(0)
	line := int32(0)
	for curI <= bci {
		firstByte, err := r.ReadByte()
		if err != nil || firstByte&0x80 == 0 {
			log.Debugf("first byte: sync lost (%x) or error: %v",
				firstByte, err)
			return 0
		}

		code := (firstByte >> 3) & 15
		curI += uint32(firstByte&7) + 1

		// Handle the 16 possible different codes known as _PyCodeLocationInfoKind.
		//nolint:lll
		// https://github.com/python/cpython/blob/deaf509e8fc6e0363bd6f26d52ad42f976ec42f2/Include/cpython/code.h#L219
		switch code {
		case 0, 1, 2, 3, 4, 5, 6, 7, 8, 9:
			// PY_CODE_LOCATION_INFO_SHORT does not hold line information.
			_, _ = r.ReadByte()
		case 10, 11, 12:
			// PY_CODE_LOCATION_INFO_ONE_LINE embeds the line information in the code
			// follows two bytes containing new columns.
			line += int32(code - 10)
			_, _ = r.ReadByte()
			_, _ = r.ReadByte()
		case 13:
			// PY_CODE_LOCATION_INFO_NO_COLUMNS
			line += readSignedVarint(r)
		case 14:
			// PY_CODE_LOCATION_INFO_LONG
			line += readSignedVarint(r)
			_ = readVarint(r)
			_ = readVarint(r)
			_ = readVarint(r)
		case 15:
			// PY_CODE_LOCATION_INFO_NONE does not hold line information
			line = -1
		default:
			log.Debugf("Unexpected PyCodeLocationInfoKind %d", code)
			return 0
		}
	}
	if line < 0 {
		line = 0
	}
	return uint32(line)
}

// walkLineTable implements the algorithm to walk the line number table that was introduced
// with Python 3.10. While firstLineNo still holds the line number of the function, the line
// number table extends this information with the offset into this function.
func walkLineTable(m *pythonCodeObject, addrq uint32) uint32 {
	// The co_linetab format is specified in python Objects/lnotab_notes.txt
	if addrq == 0 {
		return 0
	}
	lineTable := m.lineTable
	var line, start, end uint32
	for i := 0; i < len(lineTable)/2; i += 2 {
		sDelta := lineTable[i]
		lDelta := int8(lineTable[i+1])
		if lDelta == 0 {
			end += uint32(sDelta)
			continue
		}
		start = end
		end = start + uint32(sDelta)
		if lDelta == -128 {
			// A line delta of -128 is a special indicator mentioned in
			// Objects/lnotab_notes.txt and indicates an invalid line number.
			continue
		}
		line += uint32(lDelta)
		if end == start {
			continue
		}
		if end > addrq {
			return line
		}
	}
	return 0
}

func mapByteCodeIndexToLine(m *pythonCodeObject, bci uint32) uint32 {
	// The co_lntab format is specified in python Objects/lnotab_notes.txt
	lineno := uint32(0)
	addr := uint(0)
	// The lnotab length is checked to be even before it's extracted in getCodeObject()
	lnotab := m.lineTable
	for i := 0; i < len(lnotab); i += 2 {
		addr += uint(lnotab[i])
		if addr > uint(bci) {
			return lineno
		}
		lineno += uint32(lnotab[i+1])
		if lnotab[i+1] >= 0x80 {
			lineno -= 0x100
		}
	}
	return lineno
}

// getFuncOffsetFunc provides functionality to return a function offset from a PyCodeObject
type getFuncOffsetFunc func(m *pythonCodeObject, bci uint32) uint32

type pythonInstance struct {
	interpreter.InstanceStubs

	// Python symbolization metrics
	successCount atomic.Uint64
	failCount    atomic.Uint64

	d    *pythonData
	rm   remotememory.RemoteMemory
	bias libpf.Address

	// addrToCodeObject maps a Python Code object to a pythonCodeObject which caches
	// the needed data from it.
	addrToCodeObject *freelru.LRU[libpf.Address, *pythonCodeObject]

	// getFuncOffset provides fast access in order to get the function offset for different
	// Python interpreter versions.
	getFuncOffset getFuncOffsetFunc

	// procInfoInserted tracks whether we've already inserted process info into BPF maps.
	procInfoInserted bool
}

var _ interpreter.Instance = &pythonInstance{}

func (p *pythonInstance) GetAndResetMetrics() ([]metrics.Metric, error) {
	addrToCodeObjectStats := p.addrToCodeObject.ResetMetrics()

	return []metrics.Metric{
		{
			ID:    metrics.IDPythonSymbolizationSuccesses,
			Value: metrics.MetricValue(p.successCount.Swap(0)),
		},
		{
			ID:    metrics.IDPythonSymbolizationFailures,
			Value: metrics.MetricValue(p.failCount.Swap(0)),
		},
		{
			ID:    metrics.IDPythonAddrToCodeObjectHit,
			Value: metrics.MetricValue(addrToCodeObjectStats.Hits),
		},
		{
			ID:    metrics.IDPythonAddrToCodeObjectMiss,
			Value: metrics.MetricValue(addrToCodeObjectStats.Misses),
		},
		{
			ID:    metrics.IDPythonAddrToCodeObjectAdd,
			Value: metrics.MetricValue(addrToCodeObjectStats.Inserts),
		},
		{
			ID:    metrics.IDPythonAddrToCodeObjectDel,
			Value: metrics.MetricValue(addrToCodeObjectStats.Removals),
		},
	}, nil
}

func (p *pythonInstance) UpdateTSDInfo(ebpf interpreter.EbpfHandler, pid libpf.PID,
	tsdInfo tpbase.TSDInfo) error {
	d := p.d
	vm := &d.vmStructs
	cdata := support.PyProcInfo{
		AutoTLSKeyAddr: uint64(d.autoTLSKey) + uint64(p.bias),
		NoneStructAddr: uint64(d.noneStruct) + uint64(p.bias),
		Version:        d.version,

		TsdInfo: support.TSDInfo{
			Offset:     tsdInfo.Offset,
			Multiplier: tsdInfo.Multiplier,
			Indirect:   tsdInfo.Indirect,
		},

		PyThreadState_frame:            uint8(vm.PyThreadState.Frame),
		PyCFrame_current_frame:         uint8(vm.PyCFrame.CurrentFrame),
		PyFrameObject_f_back:           uint8(vm.PyFrameObject.Back),
		PyFrameObject_f_code:           uint8(vm.PyFrameObject.Code),
		PyFrameObject_f_lasti:          uint8(vm.PyFrameObject.LastI),
		PyFrameObject_entry_member:     uint8(vm.PyFrameObject.EntryMember),
		PyFrameObject_entry_val:        uint8(vm.PyFrameObject.EntryVal),
		PyCodeObject_co_argcount:       uint8(vm.PyCodeObject.ArgCount),
		PyCodeObject_co_kwonlyargcount: uint8(vm.PyCodeObject.KwOnlyArgCount),
		PyCodeObject_co_flags:          uint8(vm.PyCodeObject.Flags),
		PyCodeObject_co_firstlineno:    uint8(vm.PyCodeObject.FirstLineno),
		PyCodeObject_sizeof:            uint8(vm.PyCodeObject.Sizeof),
		Continue_with_next_unwinder:    uint8(continueWithNextUnwinder()),
	}

	err := ebpf.UpdateProcData(libpf.Python, pid, unsafe.Pointer(&cdata))
	if err != nil {
		return err
	}

	p.procInfoInserted = true
	return err
}

func (p *pythonInstance) Detach(ebpf interpreter.EbpfHandler, pid libpf.PID) error {
	if !p.procInfoInserted {
		return nil
	}

	err := ebpf.DeleteProcData(libpf.Python, pid)
	if err != nil {
		return fmt.Errorf("failed to detach pythonInstance from PID %d: %v",
			pid, err)
	}
	return nil
}

// frozenNameToFileName convert special Python file names into real file names.
// Return the new file name or the unchanged input if it wasn't a frozen file name
// or the format was not as expected.
//
// Examples seen regularly with python3.7 and python3.8:
//
//	"<frozen importlib._bootstrap>" --> "_bootstrap.py"
//	"<frozen importlib._bootstrap_external>" --> "_bootstrap_external.py"
func frozenNameToFileName(sourceFileName string) (string, error) {
	if !strings.HasPrefix(sourceFileName, "<frozen ") {
		return sourceFileName, nil
	}

	if sourceFileName[len(sourceFileName)-1] != '>' {
		return "", fmt.Errorf("missing terminator in frozen file '%s'", sourceFileName)
	}

	b := strings.LastIndexByte(sourceFileName, '.') + 1
	if b == 0 {
		b = 8 // advance to file name, starting after '<frozen '
	}

	fName := sourceFileName[b : len(sourceFileName)-1]
	if fName == "" {
		return "", fmt.Errorf("unexpected empty frozen file '%s'", sourceFileName)
	}

	return fName + ".py", nil
}

func (p *pythonInstance) getCodeObject(addr libpf.Address,
	ebpfChecksum uint32) (*pythonCodeObject, error) {
	if addr == 0 {
		return nil, errors.New("failed to read code object: null pointer")
	}
	if value, ok := p.addrToCodeObject.Get(addr); ok {
		m := value
		if m.ebpfChecksum == ebpfChecksum {
			return m, nil
		}
	}

	vms := &p.d.vmStructs
	cobj := make([]byte, vms.PyCodeObject.Sizeof)
	if err := p.rm.Read(addr, cobj); err != nil {
		return nil, fmt.Errorf("failed to read code object: %v", err)
	}

	// Parse the PyCodeObject structure
	firstLineNo := npsr.Uint32(cobj, vms.PyCodeObject.FirstLineno)
	argCount := npsr.Uint32(cobj, vms.PyCodeObject.ArgCount)
	kwonlyArgCount := npsr.Uint32(cobj, vms.PyCodeObject.KwOnlyArgCount)
	flags := npsr.Uint32(cobj, vms.PyCodeObject.Flags)
	data := libpf.Address(vms.PyASCIIObject.Data)

	var lineInfoPtr libpf.Address
	if p.d.version < pythonVer(3, 10) {
		lineInfoPtr = npsr.Ptr(cobj, vms.PyCodeObject.Lnotab)
	} else {
		lineInfoPtr = npsr.Ptr(cobj, vms.PyCodeObject.Linetable)
	}

	var name string
	if vms.PyCodeObject.QualName != 0 {
		name = p.rm.String(data + npsr.Ptr(cobj, vms.PyCodeObject.QualName))
	}
	if name == "" {
		name = p.rm.String(data + npsr.Ptr(cobj, vms.PyCodeObject.Name))
	}
	if !util.IsValidString(name) {
		log.Debugf("Extracted invalid Python method/function name at 0x%x '%v'",
			addr, []byte(name))
		return nil, fmt.Errorf("extracted invalid Python method/function name from address 0x%x",
			addr)
	}

	sourcePath := p.rm.String(data + npsr.Ptr(cobj, vms.PyCodeObject.Filename))
	sourceFileName := ""

	// Correct frozen files to be displayed correctly in the UI
	sourceFileName, err := frozenNameToFileName(sourcePath)
	if err != nil {
		sourceFileName = sourcePath
	}
	if !util.IsValidString(sourceFileName) {
		log.Debugf("Extracted invalid Python source file name at 0x%x '%v'",
			addr, []byte(sourceFileName))
		return nil, fmt.Errorf("extracted invalid Python source file name from address 0x%x",
			addr)
	}

	ebpfChecksumCalculated := (argCount << 25) + (kwonlyArgCount << 18) +
		(flags << 10) + firstLineNo
	if ebpfChecksum != ebpfChecksumCalculated {
		return nil, fmt.Errorf("read code object was stale: %x != %x",
			ebpfChecksum, ebpfChecksumCalculated)
	}

	lineTableSize := p.rm.Uint64(lineInfoPtr + libpf.Address(vms.PyVarObject.ObSize))
	if lineTableSize >= 0x10000 || (p.d.version < pythonVer(3, 11) && lineTableSize&1 != 0) {
		return nil, fmt.Errorf("invalid line table size (%v)", lineTableSize)
	}
	lineTable := make([]byte, lineTableSize)
	err = p.rm.Read(lineInfoPtr+libpf.Address(vms.PyBytesObject.Sizeof)-1, lineTable)
	if err != nil {
		return nil, fmt.Errorf("failed to read line table: %v", err)
	}

	pco := &pythonCodeObject{
		version:        p.d.version,
		name:           libpf.Intern(name),
		sourceFileName: libpf.Intern(sourceFileName),
		firstLineNo:    firstLineNo,
		lineTable:      lineTable,
		ebpfChecksum:   ebpfChecksum,
	}
	p.addrToCodeObject.Add(addr, pco)
	return pco, nil
}

func (p *pythonInstance) Symbolize(frame *host.Frame, frames *libpf.Frames) error {
	if !frame.Type.IsInterpType(libpf.Python) {
		return interpreter.ErrMismatchInterpreterType
	}

	// Extract the Python frame bitfields from the file and line variables
	ptr := libpf.Address(frame.File)
	lastI := uint32(frame.Lineno>>32) & 0x0fffffff
	objectID := uint32(frame.Lineno)

	sfCounter := successfailurecounter.New(&p.successCount, &p.failCount)
	defer sfCounter.DefaultToFailure()

	// Extract and symbolize
	m, err := p.getCodeObject(ptr, objectID)
	if err != nil {
		return fmt.Errorf("failed to get python object %x: %v", objectID, err)
	}

	functionOffset := p.getFuncOffset(m, lastI)
	frames.Append(&libpf.Frame{
		Type:           libpf.PythonFrame,
		FunctionName:   m.name,
		SourceFile:     m.sourceFileName,
		SourceLine:     libpf.SourceLineno(m.firstLineNo + functionOffset),
		FunctionOffset: functionOffset,
	})

	sfCounter.ReportSuccess()
	return nil
}

// fieldByPythonName searches obj for a field by its Python name using the struct tags.
func fieldByPythonName(obj reflect.Value, fieldName string) reflect.Value {
	objType := obj.Type()
	for i := 0; i < obj.NumField(); i++ {
		objField := objType.Field(i)
		if nameTag, ok := objField.Tag.Lookup("name"); ok {
			if slices.Contains(strings.Split(nameTag, ","), fieldName) {
				return obj.Field(i)
			}
		}
		if fieldName == objField.Name {
			return obj.Field(i)
		}
	}
	return reflect.Value{}
}

func (d *pythonData) readIntrospectionData(ef *pfelf.File, symbol libpf.SymbolName,
	vmObj any) error {
	typeData, err := ef.LookupSymbolAddress(symbol)
	if err != nil {
		return fmt.Errorf("symbol '%s' not found", symbol)
	}
	rm := ef.GetRemoteMemory()
	vms := &d.vmStructs
	typedataAddress := libpf.Address(typeData)
	reflection := reflect.ValueOf(vmObj).Elem()
	if f := reflection.FieldByName("Sizeof"); f.IsValid() {
		size := rm.Uint64(typedataAddress + vms.PyTypeObject.BasicSize)
		f.SetUint(size)
	}

	membersPtr := rm.Ptr(typedataAddress + vms.PyTypeObject.Members)
	if membersPtr == 0 {
		return nil
	}

	for addr := membersPtr; true; addr += vms.PyMemberDef.Sizeof {
		memberName := rm.StringPtr(addr + libpf.Address(vms.PyMemberDef.Name))
		if memberName == "" {
			break
		}
		if f := fieldByPythonName(reflection, memberName); f.IsValid() {
			offset := rm.Uint32(addr + libpf.Address(vms.PyMemberDef.Offset))
			f.SetUint(uint64(offset))
		}
	}
	return nil
}

// decodeStub will resolve a given symbol, extract the code for it, and analyze
// the code to resolve specified argument parameter to the first jump/call.
func decodeStub(ef *pfelf.File, memoryBase libpf.SymbolValue,
	symbolName libpf.SymbolName) (libpf.SymbolValue, error) {
	// Read and decode the code for the symbol
	sym, code, err := ef.SymbolData(symbolName, 64)
	if err != nil {
		return libpf.SymbolValueInvalid, fmt.Errorf("unable to read '%s': %v",
			symbolName, err)
	}
	var value libpf.SymbolValue
	switch ef.Machine {
	case elf.EM_AARCH64:
		value, err = decodeStubArgumentARM64(code, memoryBase), nil
	case elf.EM_X86_64:
		value, err = decodeStubArgumentAMD64(code, uint64(sym.Address), uint64(memoryBase))
	default:
		return libpf.SymbolValueInvalid, fmt.Errorf("unsupported arch %s", ef.Machine.String())
	}

	// Sanity check the value range and alignment
	if err != nil || value%4 != 0 {
		return libpf.SymbolValueInvalid, fmt.Errorf("decode stub %s 0x%x %s failed (0x%x):  %v",
			symbolName, sym.Address, hex.Dump(code), value, err)
	}
	// If base symbol (_PyRuntime) is not provided, accept any found value.
	if memoryBase == 0 && value != 0 {
		return value, nil
	}
	// Check that the found value is within reasonable distance from the given symbol.
	if value > memoryBase && value < memoryBase+4096 {
		return value, nil
	}
	return libpf.SymbolValueInvalid, fmt.Errorf("decode stub %s 0x%x %s failed (0x%x)",
		symbolName, sym.Address, hex.Dump(code), value)
}

func Loader(ebpf interpreter.EbpfHandler, info *interpreter.LoaderInfo) (interpreter.Data, error) {
	mainDSO := false
	matches := libpythonRegex.FindStringSubmatch(info.FileName())
	if matches == nil {
		mainDSO = true
		matches = pythonRegex.FindStringSubmatch(info.FileName())
		if matches == nil {
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
		if slices.ContainsFunc(needed, libpythonRegex.MatchString) {
			// 'python' linked with 'libpython'. The beef is in the library,
			// so do not try to inspect the shim main binary.
			return nil, nil
		}
	}

	var pyruntimeAddr, autoTLSKey libpf.SymbolValue
	major, _ := strconv.Atoi(matches[1])
	minor, _ := strconv.Atoi(matches[2])
	version := pythonVer(major, minor)

	minVer := pythonVer(3, 6)
	maxVer := pythonVer(3, 13)
	if version < minVer || version > maxVer {
		return nil, fmt.Errorf("unsupported Python %d.%d (need >= %d.%d and <= %d.%d)",
			major, minor,
			(minVer>>8)&0xff, minVer&0xff,
			(maxVer>>8)&0xff, maxVer&0xff)
	}

	if version >= pythonVer(3, 7) {
		if pyruntimeAddr, err = ef.LookupSymbolAddress("_PyRuntime"); err != nil {
			return nil, fmt.Errorf("_PyRuntime not defined: %v", err)
		}
	}

	// Calls first: PyThread_tss_get(autoTSSKey)
	autoTLSKey, err = decodeStub(ef, pyruntimeAddr, "PyGILState_GetThisThreadState")
	if autoTLSKey == libpf.SymbolValueInvalid {
		return nil, fmt.Errorf("unable to resolve autoTLSKey %v", err)
	}
	if version >= pythonVer(3, 7) && autoTLSKey%8 == 0 {
		// On Python 3.7+, the call is to PyThread_tss_get, but can get optimized to
		// call directly pthread_getspecific. So we might be finding the address
		// for "Py_tss_t" or "pthread_key_t" depending on call target.
		// Technically it would be best to resolve the jmp/call destination, but
		// finding the jump slot name requires fairly complex plt relocation parsing.
		// Instead this assumes that the TLS key address should be addr%8==4. This
		// is because Py_tss_t consists of two "int" types and we want the latter.
		// The first "int" is guaranteed to be aligned to 8, because in struct _PyRuntime
		// it follows a pointer field.
		autoTLSKey += 4
	}

	interpRanges, err := findInterpreterRanges(info, ef)
	if err != nil {
		return nil, err
	}

	pd := &pythonData{
		version:    version,
		autoTLSKey: autoTLSKey,
	}
	vms := &pd.vmStructs

	if version >= pythonVer(3, 13) {
		// CPython commit 7199584ac8632eab57612f595a7162ab8d2ebbc0 makes
		// `f_executable` (referred to here as `f_code` in most places) point to
		// `Py_None` for the top level frame that's entered from the C side,
		// while before it was pointing to the trampoline, which was a valid
		// code object.
		//
		// `Py_None` obviously is not a valid code object, and if we try to
		// decode it we get bad names / lines tables, so we need to identify
		// this case, and stop unwinding on the eBPF side.
		//
		// Detecting `Py_None` is quite simple, it's the address of the exported
		// `_Py_NoneStruct` symbol, so we get that address here and pass it to
		// eBPF, so it knows when to stop.
		if pd.noneStruct, err = ef.LookupSymbolAddress("_Py_NoneStruct"); err != nil {
			return nil, fmt.Errorf("_Py_NoneStruct not defined: %v", err)
		}
	}

	// Introspection data not available for these structures
	vms.PyTypeObject.BasicSize = 32
	vms.PyTypeObject.Members = 240
	vms.PyMemberDef.Name = 0
	vms.PyMemberDef.Offset = 16
	vms.PyMemberDef.Sizeof = 40

	vms.PyASCIIObject.Data = 48
	vms.PyVarObject.ObSize = 16
	vms.PyThreadState.Frame = 24

	switch version {
	case pythonVer(3, 11):
		// Starting with 3.11 we no longer can extract needed information from
		// PyFrameObject. In addition PyFrameObject was replaced with _PyInterpreterFrame.
		// The following offsets come from _PyInterpreterFrame but we continue to use
		// PyFrameObject as the structure name, since the struct elements serve the same
		// function as before.
		vms.PyFrameObject.Code = 32
		vms.PyFrameObject.LastI = 56       // _Py_CODEUNIT *prev_instr
		vms.PyFrameObject.Back = 48        // struct _PyInterpreterFrame *previous
		vms.PyFrameObject.EntryMember = 68 // bool is_entry
		vms.PyFrameObject.EntryVal = 1     // true, from stdbool.h
		// frame got removed in PyThreadState but we can use cframe instead.
		vms.PyThreadState.Frame = 56
		vms.PyCFrame.CurrentFrame = 8
	case pythonVer(3, 12):
		// Entry frame detection changed due to the shim frame
		// https://github.com/python/cpython/commit/1e197e63e21f77b102ff2601a549dda4b6439455
		vms.PyFrameObject.Code = 0
		vms.PyFrameObject.LastI = 56       // _Py_CODEUNIT *prev_instr
		vms.PyFrameObject.Back = 8         // struct _PyInterpreterFrame *previous
		vms.PyFrameObject.EntryMember = 70 // char owner
		vms.PyFrameObject.EntryVal = 3     // enum _frameowner, FRAME_OWNED_BY_CSTACK
		vms.PyThreadState.Frame = 56
		vms.PyCFrame.CurrentFrame = 0
		vms.PyASCIIObject.Data = 40
	case pythonVer(3, 13):
		vms.PyFrameObject.Code = 0
		vms.PyFrameObject.LastI = 56       // _Py_CODEUNIT *prev_instr
		vms.PyFrameObject.Back = 8         // struct _PyInterpreterFrame *previous
		vms.PyFrameObject.EntryMember = 70 // char owner
		vms.PyFrameObject.EntryVal = 3     // enum _frameowner, FRAME_OWNED_BY_CSTACK
		vms.PyThreadState.Frame = 72
		// Current frame is not used anymore, see commit 006e44f9 in the CPython repo:
		// they removed one level of indirection.
		vms.PyCFrame.CurrentFrame = 0
		vms.PyASCIIObject.Data = 40
	}

	// Read the introspection data from objects types that have it
	if err := pd.readIntrospectionData(ef, "PyCode_Type", &vms.PyCodeObject); err != nil {
		return nil, err
	}
	if err := pd.readIntrospectionData(ef, "PyFrame_Type", &vms.PyFrameObject); err != nil {
		return nil, err
	}
	if err := pd.readIntrospectionData(ef, "PyBytes_Type", &vms.PyBytesObject); err != nil {
		return nil, err
	}

	if err := ebpf.UpdateInterpreterOffsets(support.ProgUnwindPython, info.FileID(),
		interpRanges); err != nil {
		return nil, err
	}

	return pd, nil
}

func findInterpreterRanges(info *interpreter.LoaderInfo, ef *pfelf.File,
) (interpRanges []util.Range, err error) {
	// The Python main interpreter loop history in CPython git is:
	//
	//nolint:lll
	// 87af12bff33 v3.11 2022-02-15 _PyEval_EvalFrameDefault(PyThreadState*,_PyInterpreterFrame*,int)
	// ae0a2b75625 v3.10 2021-06-25 _PyEval_EvalFrameDefault(PyThreadState*,_interpreter_frame*,int)
	// 0b72b23fb0c v3.9  2020-03-12 _PyEval_EvalFrameDefault(PyThreadState*,PyFrameObject*,int)
	// 3cebf938727 v3.6  2016-09-05 _PyEval_EvalFrameDefault(PyFrameObject*,int)
	// 49fd7fa4431 v3.0  2006-04-21 PyEval_EvalFrameEx(PyFrameObject*,int)
	var interp *libpf.Symbol
	var code []byte
	const maxCodeSize = 128 * 1024 // observed ~65k in the wild
	if interp, code, err = ef.SymbolData("_PyEval_EvalFrameDefault", maxCodeSize); err != nil {
		interp, code, err = ef.SymbolData("PyEval_EvalFrameEx", maxCodeSize)
	}
	if err != nil {
		return nil, errors.New("no _PyEval_EvalFrameDefault/PyEval_EvalFrameEx symbol found")
	}
	interpRanges = make([]util.Range, 0, 2)
	interpRanges = append(interpRanges, util.Range{
		Start: uint64(interp.Address),
		End:   uint64(interp.Address) + interp.Size,
	})
	coldRange, err := findColdRange(ef, code, interp)
	if err != nil {
		log.WithError(err).Warnf("failed to recover python ranges %s",
			info.FileName())
	}
	if coldRange != (util.Range{}) {
		interpRanges = append(interpRanges, coldRange)
	}
	return interpRanges, nil
}

// findColdRange finds a relative jump from the _PyEval_EvalFrameDefault outside itself
// (to _PyEval_EvalFrameDefault.cold symbol) and then recovers the range of the .cold
// symbol using an instance of elfunwindinfo.EhFrameTable.
// findColdRange returns the util.Range of the `.cold` symbol or an empty util.Range
// https://github.com/open-telemetry/opentelemetry-ebpf-profiler/issues/416
func findColdRange(ef *pfelf.File, code []byte, interp *libpf.Symbol) (util.Range, error) {
	if ef.Machine != elf.EM_X86_64 {
		return util.Range{}, nil
	}
	dst, err := amd.FindExternalJump(code, interp)
	if err != nil || dst == 0 {
		return util.Range{}, err
	}
	t, err := elfunwindinfo.NewEhFrameTable(ef)
	if err != nil {
		return util.Range{}, err
	}
	fde, err := t.LookupFDE(dst)
	if err != nil {
		return util.Range{}, err
	}
	return util.Range{
		Start: uint64(fde.PCBegin),
		End:   uint64(fde.PCBegin + fde.PCRange),
	}, nil
}
