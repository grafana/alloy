// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package hotspot // import "go.opentelemetry.io/ebpf-profiler/interpreter/hotspot"

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/elastic/go-freelru"

	"go.opentelemetry.io/ebpf-profiler/interpreter"
	"go.opentelemetry.io/ebpf-profiler/libpf"
	"go.opentelemetry.io/ebpf-profiler/libpf/pfelf"
	"go.opentelemetry.io/ebpf-profiler/libpf/xsync"
	"go.opentelemetry.io/ebpf-profiler/lpm"
	npsr "go.opentelemetry.io/ebpf-profiler/nopanicslicereader"
	"go.opentelemetry.io/ebpf-profiler/remotememory"
)

// hotspotIntrospectionTable contains the resolved ELF symbols for an introspection table
type hotspotIntrospectionTable struct {
	skipBaseDref               bool
	base, stride               libpf.Address
	typeOffset, fieldOffset    libpf.Address
	valueOffset, addressOffset libpf.Address
}

// resolveSymbols resolves the ELF symbols of the introspection table
func (it *hotspotIntrospectionTable) resolveSymbols(ef *pfelf.File, symNames []string) error {
	symVals := make([]libpf.Address, len(symNames))
	for i, s := range symNames {
		if s == "" {
			continue
		}
		addr, err := ef.LookupSymbolAddress(libpf.SymbolName(s))
		if err != nil {
			return fmt.Errorf("symbol '%v' not found: %w", s, err)
		}
		symVals[i] = libpf.Address(addr)
	}

	it.base, it.stride = symVals[0], symVals[1]
	it.typeOffset, it.fieldOffset = symVals[2], symVals[3]
	it.valueOffset, it.addressOffset = symVals[4], symVals[5]
	return nil
}

// hotspotVMData contains static information from one HotSpot build (libjvm.so).
// It mostly is limited to the introspection data (class sizes and field offsets) and
// the version.
type hotspotVMData struct {
	// err is the permanent error if introspection data is not supported
	err error

	// version is the JDK numeric version. Used in some places to make version specific
	// adjustments to the unwinding process.
	version uint32

	// versionStr is the Hotspot build version string, and can contain additional
	// details such as the distribution name and patch level.
	versionStr string

	// unsigned5X is the number of exclusion bytes used in UNSIGNED5 encoding
	unsigned5X uint8

	// nmethodUsesOffsets is set if the nmethod code start/end and deoptimize handler
	// are offsets (instead of pointers)
	nmethodUsesOffsets uint8

	// vmStructs reflects the HotSpot introspection data we want to extract
	// from the runtime. It is filled using golang reflection (the struct and
	// field names are used to find the data from the JVM). Thus the structs
	// here are following the JVM naming convention.
	//
	// The comments of .Sizeof like ">xxx" are to signify the size range of the JVM
	// C++ class  and thus the expected value of .Sizeof member. This is mainly to
	// indicate the classes for which uint8 is not enough to hold the offset values
	// for the eBPF code.
	//nolint:lll
	vmStructs struct {
		AbstractVMVersion struct {
			Release         libpf.Address `name:"_s_vm_release"`
			MajorVersion    libpf.Address `name:"_vm_major_version"`
			MinorVersion    libpf.Address `name:"_vm_minor_version"`
			SecurityVersion libpf.Address `name:"_vm_security_version"`
			BuildNumber     libpf.Address `name:"_vm_build_number"`
		} `name:"Abstract_VM_Version"`
		JdkVersion struct {
			Current libpf.Address `name:"_current"`
		} `name:"JDK_Version"`
		CodeBlob struct {
			Sizeof              uint
			Name                uint `name:"_name"`
			FrameCompleteOffset uint `name:"_frame_complete_offset"`
			FrameSize           uint `name:"_frame_size"`
			// JDK -8: offset, JDK 9+: pointers, JDK 23+: offset
			CodeBegin uint `name:"_code_begin,_code_offset"`
			CodeEnd   uint `name:"_code_end,_data_offset"`
			// Needed for JDK23+ for layout calculation
			Size uint `name:"_size"`
			// JDK25+: the metadata is inside mutable data
			RelocationSize  uint `name:"_relocation_size"`
			MutableDataSize uint `name:"_mutable_data_size"`
			MutableData     uint `name:"_mutable_data"`
		}
		CodeCache struct {
			Heap      libpf.Address `name:"_heap"`
			Heaps     libpf.Address `name:"_heaps"`
			HighBound libpf.Address `name:"_high_bound"`
			LowBound  libpf.Address `name:"_low_bound"`
		}
		CodeHeap struct {
			Sizeof          uint
			Log2SegmentSize uint `name:"_log2_segment_size"`
			Memory          uint `name:"_memory"`
			Segmap          uint `name:"_segmap"`
		}
		ConstantPool struct {
			Sizeof              uint
			PoolHolder          uint `name:"_pool_holder"`
			SourceFileNameIndex uint `name:"_source_file_name_index"`
		} `name:"ConstantPool,constantPoolOopDesc"`
		ConstMethod struct {
			Sizeof    uint
			Constants uint `name:"_constants"`
			CodeSize  uint `name:"_code_size"`
			// JDK21+: ConstMethod._flags is now a struct with another _flags field
			// https://github.com/openjdk/jdk/commit/316d303c1da550c9589c9be56b65650964e3886b
			Flags          uint `name:"_flags,_flags._flags"`
			NameIndex      uint `name:"_name_index"`
			SignatureIndex uint `name:"_signature_index"`
		} `name:"ConstMethod,constMethodOopDesc"`
		// JDK9-15 structure
		GenericGrowableArray struct {
			Len uint `name:"_len"`
		}
		// JDK16 structure
		GrowableArrayBase struct {
			Len uint `name:"_len"`
		}
		GrowableArrayInt struct {
			Sizeof uint
			Data   uint `name:"_data"`
		} `name:"GrowableArray<int>"`
		HeapBlock struct {
			Sizeof uint
		}
		InstanceKlass struct { // .Sizeof >400
			Sizeof              uint
			SourceFileNameIndex uint `name:"_source_file_name_index"`
			SourceFileName      uint `name:"_source_file_name"` // JDK -7 only
		} `name:"InstanceKlass,instanceKlass"`
		Klass struct { // .Sizeof >200
			Sizeof uint
			Name   uint `name:"_name"`
		}
		Method struct {
			ConstMethod uint `name:"_constMethod"`
		} `name:"Method,methodOopDesc"`
		Nmethod struct { // .Sizeof >256
			Sizeof             uint
			CompileID          uint `name:"_compile_id"`
			MetadataOffset     uint `name:"_metadata_offset,_oops_offset"`
			ScopesPcsOffset    uint `name:"_scopes_pcs_offset"`
			DependenciesOffset uint `name:"_dependencies_offset"` // JDK -22 only
			ImmutableData      uint `name:"_immutable_data"`      // JDK 23+ only
			ImmutableDataSize  uint `name:"_immutable_data_size"` // JDK 23+ only
			OrigPcOffset       uint `name:"_orig_pc_offset"`
			DeoptimizeOffset   uint `name:"_deoptimize_offset,_deopt_handler_offset,_deopt_handler_begin"`
			Method             uint `name:"_method"`
			ScopesDataOffset   uint `name:"_scopes_data_offset,_scopes_data_begin"`
		} `name:"nmethod,CompiledMethod"`
		OopDesc struct {
			Sizeof uint
		} `name:"oopDesc"`
		PcDesc struct {
			Sizeof            uint
			PcOffset          uint `name:"_pc_offset"`
			ScopeDecodeOffset uint `name:"_scope_decode_offset"`
		}
		StubRoutines struct {
			Sizeof   uint                     // not needed, just keep this out of CatchAll
			CatchAll map[string]libpf.Address `name:"*"`
		}
		Symbol struct {
			Sizeof            uint
			Body              uint `name:"_body"`
			Length            uint `name:"_length"`
			LengthAndRefcount uint `name:"_length_and_refcount"`
		}
		VirtualSpace struct {
			HighBoundary uint `name:"_high_boundary"`
			LowBoundary  uint `name:"_low_boundary"`
		}
	}
}

// fieldByJavaName searches obj for a field by its JVM name using the struct tags.
func fieldByJavaName(obj reflect.Value, fieldName string) reflect.Value {
	var catchAll reflect.Value

	objType := obj.Type()
	for i := 0; i < obj.NumField(); i++ {
		objField := objType.Field(i)
		if nameTag, ok := objField.Tag.Lookup("name"); ok {
			for _, javaName := range strings.Split(nameTag, ",") {
				if fieldName == javaName {
					return obj.Field(i)
				}
				if javaName == "*" {
					catchAll = obj.Field(i)
				}
			}
		}
		if fieldName == objField.Name {
			return obj.Field(i)
		}
	}

	return catchAll
}

// parseIntrospection loads and parses HotSpot introspection tables. It will then fill in
// hotspotData.vmStructs using reflection to gather the offsets and sizes
// we are interested about.
func (vmd *hotspotVMData) parseIntrospection(it *hotspotIntrospectionTable,
	rm remotememory.RemoteMemory, loadBias libpf.Address) error {
	stride := libpf.Address(rm.Uint64(it.stride + loadBias))
	typeOffs := uint(rm.Uint64(it.typeOffset + loadBias))
	addrOffs := uint(rm.Uint64(it.addressOffset + loadBias))
	fieldOffs := uint(rm.Uint64(it.fieldOffset + loadBias))
	valOffs := uint(rm.Uint64(it.valueOffset + loadBias))
	base := it.base + loadBias

	if !it.skipBaseDref {
		base = rm.Ptr(base)
	}

	if base == 0 || stride == 0 {
		return fmt.Errorf("bad introspection table data (%#x / %d)", base, stride)
	}

	// Parse the introspection table
	e := make([]byte, stride)
	vm := reflect.ValueOf(&vmd.vmStructs).Elem()
	for addr := base; true; addr += stride {
		if err := rm.Read(addr, e); err != nil {
			return err
		}

		typeNamePtr := npsr.Ptr(e, typeOffs)
		if typeNamePtr == 0 {
			break
		}

		typeName := rm.String(typeNamePtr)
		f := fieldByJavaName(vm, typeName)
		if !f.IsValid() {
			continue
		}

		// If parsing the Types table, we have sizes. Otherwise, we are
		// parsing offsets for fields.
		fieldName := "Sizeof"
		if it.fieldOffset != 0 {
			fieldNamePtr := npsr.Ptr(e, fieldOffs)
			fieldName = rm.String(fieldNamePtr)
			if fieldName == "" || fieldName[0] != '_' {
				continue
			}
		}

		f = fieldByJavaName(f, fieldName)
		if !f.IsValid() {
			continue
		}

		value := uint64(npsr.Ptr(e, addrOffs))
		if value != 0 {
			// We just resolved a const pointer. Adjust it by loadBias
			// to get a globally cacheable unrelocated virtual address.
			value -= uint64(loadBias)
			log.Debugf("JVM %v.%v = @ %x", typeName, fieldName, value)
		} else {
			// Literal value
			value = npsr.Uint64(e, valOffs)
			log.Debugf("JVM %v.%v = %v", typeName, fieldName, value)
		}

		switch f.Kind() {
		case reflect.Uint64, reflect.Uint, reflect.Uintptr:
			f.SetUint(value)
		case reflect.Map:
			if f.IsNil() {
				// maps need explicit init (nil is invalid)
				f.Set(reflect.MakeMap(f.Type()))
			}

			castedValue := reflect.ValueOf(value).Convert(f.Type().Elem())
			f.SetMapIndex(reflect.ValueOf(fieldName), castedValue)
		default:
			panic(fmt.Sprintf("bug: unexpected field type in vmStructs: %v", f.Kind()))
		}
	}
	return nil
}

type hotspotData struct {
	// ELF symbols needed for the introspection data
	typePtrs, structPtrs, jvmciStructPtrs hotspotIntrospectionTable

	// Once protected hotspotVMData
	xsync.Once[hotspotVMData]
}

func (d *hotspotData) newUnsigned5Decoder(r io.ByteReader) *unsigned5Decoder {
	return &unsigned5Decoder{
		r: r,
		x: d.Get().unsigned5X,
	}
}

func (d *hotspotData) String() string {
	if vmd := d.Get(); vmd != nil {
		return fmt.Sprintf("Java HotSpot VM %d.%d.%d+%d (%v)",
			(vmd.version>>24)&0xff, (vmd.version>>16)&0xff,
			(vmd.version>>8)&0xff, vmd.version&0xff,
			vmd.versionStr)
	}
	return "<unintrospected JVM>"
}

// Attach loads to the ebpf program the needed pointers and sizes to unwind given hotspot process.
// As the hotspot unwinder depends on the native unwinder, a part of the cleanup is done by the
// process manager and not the corresponding Detach() function of hotspot objects.
func (d *hotspotData) Attach(_ interpreter.EbpfHandler, _ libpf.PID, bias libpf.Address,
	rm remotememory.RemoteMemory) (ii interpreter.Instance, err error) {
	// Each function has four symbols: source filename, class name,
	// method name and signature. However, most of them are shared across
	// different methods, so assume about 2 unique symbols per function.
	addrToSymbol, err :=
		freelru.New[libpf.Address, libpf.String](2*interpreter.LruFunctionCacheSize,
			libpf.Address.Hash32)
	if err != nil {
		return nil, err
	}
	addrToMethod, err :=
		freelru.New[libpf.Address, *hotspotMethod](interpreter.LruFunctionCacheSize,
			libpf.Address.Hash32)
	if err != nil {
		return nil, err
	}
	addrToJITInfo, err :=
		freelru.New[libpf.Address, *hotspotJITInfo](interpreter.LruFunctionCacheSize,
			libpf.Address.Hash32)
	if err != nil {
		return nil, err
	}
	// In total there are about 100 to 200 intrinsics. We don't expect to encounter
	// everyone single one. So we use a small cache size here than LruFunctionCacheSize.
	addrToStubName, err := freelru.New[libpf.Address, libpf.String](128, libpf.Address.Hash32)
	if err != nil {
		return nil, err
	}

	return &hotspotInstance{
		d:              d,
		rm:             rm,
		bias:           bias,
		addrToSymbol:   addrToSymbol,
		addrToMethod:   addrToMethod,
		addrToJITInfo:  addrToJITInfo,
		addrToStubName: addrToStubName,
		prefixes:       libpf.Set[lpm.Prefix]{},
		stubs:          map[libpf.Address]StubRoutine{},
	}, nil
}

func (d *hotspotData) Unload(_ interpreter.EbpfHandler) {
}

// locateJvmciVMStructs attempts to heuristically locate the JVMCI VM structs by
// searching for references to the string `Klass_vtable_start_offset`. In all JVM
// versions >= 9.0, this corresponds to the first entry in the VM structs:
//
// https://github.com/openjdk/jdk/blob/jdk-9%2B181/hotspot/src/share/vm/jvmci/vmStructs_jvmci.cpp#L48
// https://github.com/openjdk/jdk/blob/jdk-22%2B10/src/hotspot/share/jvmci/vmStructs_jvmci.cpp#L49
//
//nolint:lll
func locateJvmciVMStructs(ef *pfelf.File) (libpf.Address, error) {
	const maxDataReadSize = 1 * 1024 * 1024   // seen in practice: 192 KiB
	const maxRodataReadSize = 4 * 1024 * 1024 // seen in practice: 753 KiB

	rodataSec := ef.Section(".rodata")
	if rodataSec == nil {
		return 0, errors.New("unable to find `.rodata` section")
	}

	rodata, err := rodataSec.Data(maxRodataReadSize)
	if err != nil {
		return 0, err
	}

	offs := bytes.Index(rodata, []byte("Klass_vtable_start_offset"))
	if offs == -1 {
		return 0, errors.New("unable to find string for heuristic")
	}

	ptr := rodataSec.Addr + uint64(offs)
	ptrEncoded := make([]byte, 8)
	binary.LittleEndian.PutUint64(ptrEncoded, ptr)

	dataSec := ef.Section(".data")
	if dataSec == nil {
		return 0, errors.New("unable to find `.data` section")
	}

	data, err := dataSec.Data(maxDataReadSize)
	if err != nil {
		return 0, err
	}

	offs = bytes.Index(data, ptrEncoded)
	if offs == -1 {
		return 0, errors.New("unable to find string pointer")
	}

	// 8 in the expression below is what we'd usually read from
	// gHotSpotVMStructEntryFieldNameOffset. This value unfortunately lives in
	// BSS, so we have no choice but to hard-code it. Fortunately enough this
	// offset hasn't changed since at least JDK 9.
	return libpf.Address(dataSec.Addr + uint64(offs) - 8), nil
}

// forEachItem walks the given struct reflection fields recursively, and calls the visitor
// function for each field item with it's value and name. This does not work with recursively
// linked structs, and is intended currently to be ran with the Hotspot's vmStructs struct only.
// Catch-all fields are ignored and skipped.
func forEachItem(prefix string, t reflect.Value, visitor func(reflect.Value, string) error) error {
	if prefix != "" {
		prefix += "."
	}
	for i := 0; i < t.NumField(); i++ {
		val := t.Field(i)
		fieldName := prefix + t.Type().Field(i).Name
		switch val.Kind() {
		case reflect.Struct:
			if err := forEachItem(fieldName, val, visitor); err != nil {
				return err
			}
		case reflect.Uint, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
			if err := visitor(val, fieldName); err != nil {
				return err
			}
		case reflect.Map:
			continue
		default:
			panic("unsupported type")
		}
	}
	return nil
}

// newVMData will read introspection data from remote process and return hotspotVMData
func (d *hotspotData) newVMData(rm remotememory.RemoteMemory, bias libpf.Address) (
	hotspotVMData, error) {
	// Initialize the data with non-zero values so it's easy to check that
	// everything got loaded (some fields will get zero values)
	vmd := hotspotVMData{}
	_ = forEachItem("", reflect.ValueOf(&vmd.vmStructs).Elem(),
		func(item reflect.Value, _ string) error {
			item.SetUint(^uint64(0))
			return nil
		})

	// First load the sizes of the classes
	if err := vmd.parseIntrospection(&d.typePtrs, rm, bias); err != nil {
		return vmd, err
	}
	// And the field offsets and static values
	if err := vmd.parseIntrospection(&d.structPtrs, rm, bias); err != nil {
		return vmd, err
	}
	if d.jvmciStructPtrs.base != 0 {
		if err := vmd.parseIntrospection(&d.jvmciStructPtrs, rm, bias); err != nil {
			return vmd, err
		}
	}

	// Failures after this point are permanent
	vms := &vmd.vmStructs
	var major, minor, security uint32
	if vms.JdkVersion.Current != ^libpf.Address(0) {
		// JDK8 and earlier do not export all Abstract_VM_Version fields
		jdkVersion := rm.Uint32(vms.JdkVersion.Current + bias)
		major = jdkVersion & 0xff
		minor = (jdkVersion >> 8) & 0xff
		security = (jdkVersion >> 16) & 0xff
		vms.AbstractVMVersion.MajorVersion = 0
		vms.AbstractVMVersion.MinorVersion = 0
		vms.AbstractVMVersion.SecurityVersion = 0
	} else {
		// JDK22+ no longer exports JDK_Version
		major = rm.Uint32(vms.AbstractVMVersion.MajorVersion + bias)
		minor = rm.Uint32(vms.AbstractVMVersion.MinorVersion + bias)
		security = rm.Uint32(vms.AbstractVMVersion.SecurityVersion + bias)
		vms.JdkVersion.Current = 0
	}
	build := rm.Uint32(vms.AbstractVMVersion.BuildNumber + bias)

	vmd.version = major<<24 + minor<<16 + security<<8 + build
	vmd.versionStr = rm.StringPtr(vms.AbstractVMVersion.Release + bias)

	// Check minimum supported version. JDK 7-22 supported.
	// Assume newer JDK works if the needed symbols are found.
	if major < 7 {
		vmd.err = fmt.Errorf("JVM version %d.%d.%d+%d (minimum is 7)",
			major, minor, security, build)
		return vmd, nil
	}

	if vms.ConstantPool.SourceFileNameIndex != ^uint(0) {
		// JDK15: Use ConstantPool.SourceFileNameIndex
		vms.InstanceKlass.SourceFileNameIndex = 0
		vms.InstanceKlass.SourceFileName = 0
	} else if vms.InstanceKlass.SourceFileNameIndex != ^uint(0) {
		// JDK8-14: Use InstanceKlass.SourceFileNameIndex
		vms.ConstantPool.SourceFileNameIndex = 0
		vms.InstanceKlass.SourceFileName = 0
	} else {
		// JDK7: File name is direct Symbol*, adjust offsets with OopDesc due
		// to the base pointer type changes
		vms.InstanceKlass.SourceFileName += vms.OopDesc.Sizeof
		if vms.Klass.Name != ^uint(0) {
			vms.Klass.Name += vms.OopDesc.Sizeof
		}
		vms.ConstantPool.SourceFileNameIndex = 0
		vms.InstanceKlass.SourceFileNameIndex = 0
	}

	// JDK-8: Only single CodeCache Heap, some CodeBlob and Nmethod changes
	if vms.CodeCache.Heap != ^libpf.Address(0) {
		// Validate values that can be missing
		vms.CodeCache.Heaps = 0
		vms.CodeCache.HighBound = 0
		vms.CodeCache.LowBound = 0
		vmd.nmethodUsesOffsets = 1
	} else {
		// Reset the compatibility symbols not needed
		vms.CodeCache.Heap = 0
	}

	// JDK12+: Use Symbol.Length_and_refcount for Symbol.Length
	if vms.Symbol.LengthAndRefcount != ^uint(0) {
		// The symbol _length was merged and renamed to _symbol_length_and_refcount.
		// Calculate the _length offset from it.
		vms.Symbol.Length = vms.Symbol.LengthAndRefcount + 2
	} else {
		// Reset the non-used symbols so the check below does not fail
		vms.Symbol.LengthAndRefcount = 0
	}

	// JDK16: use GenericGrowableArray as in JDK9-15 case
	if vms.GrowableArrayBase.Len != ^uint(0) {
		vms.GenericGrowableArray.Len = vms.GrowableArrayBase.Len
	} else {
		// Reset the non-used symbols so the check below does not fail
		vms.GrowableArrayBase.Len = 0
	}

	// JDK20+: UNSIGNED5 encoding change (since 20.0.15)
	// https://github.com/openjdk/jdk20u/commit/8d3399bf5f354931b0c62d2ed8095e554be71680
	if vmd.version >= 0x1400000f {
		vmd.unsigned5X = 1
	}

	// JDK23+21+: nmethod metadata layout changed completely
	// https://github.com/openjdk/jdk/commit/bdcc2400db63e604d76f9b5bd3c876271743f69f
	if vms.Nmethod.ImmutableData != ^uint(0) {
		vms.Nmethod.DependenciesOffset = 0
		vmd.nmethodUsesOffsets = 1
	} else if vms.Nmethod.DependenciesOffset != ^uint(0) {
		vms.Nmethod.ImmutableData = 0
		vms.Nmethod.ImmutableDataSize = 0
	}

	// JDK25+: metadata is inside mutable data
	if vms.CodeBlob.MutableData != ^uint(0) {
		vms.Nmethod.MetadataOffset = 0
	} else if vms.Nmethod.MetadataOffset != ^uint(0) {
		vms.CodeBlob.MutableData = 0
		vms.CodeBlob.MutableDataSize = 0
		vms.CodeBlob.RelocationSize = 0
	}

	// Check that all symbols got loaded from JVM introspection data
	err := forEachItem("", reflect.ValueOf(&vmd.vmStructs).Elem(),
		func(item reflect.Value, name string) error {
			switch item.Kind() {
			case reflect.Uint, reflect.Uint64, reflect.Uintptr:
				if item.Uint() != ^uint64(0) {
					return nil
				}
			case reflect.Uint32:
				if item.Uint() != uint64(^uint32(0)) {
					return nil
				}
			}
			return fmt.Errorf("JVM symbol '%v' not found", name)
		})
	if err != nil {
		vmd.err = err
		return vmd, nil
	}

	if vms.Symbol.Sizeof > 32 {
		// Additional sanity for Symbol.Sizeof which normally is
		// just 8 byte or so. The getSymbol() hard codes the first read
		// as 128 bytes and it needs to be more than this.
		vmd.err = fmt.Errorf("JVM Symbol.Sizeof value %d", vms.Symbol.Sizeof)
		return vmd, nil
	}

	// Verify that all struct fields are within limits
	structs := reflect.ValueOf(&vmd.vmStructs).Elem()
	for i := 0; i < structs.NumField(); i++ {
		klass := structs.Field(i)
		sizeOf := klass.FieldByName("Sizeof")
		if !sizeOf.IsValid() {
			continue
		}
		maxOffset := sizeOf.Uint()
		for j := 0; j < klass.NumField(); j++ {
			field := klass.Field(j)
			if field.Kind() == reflect.Map {
				continue
			}

			if field.Uint() > maxOffset {
				vmd.err = fmt.Errorf("%s.%s offset %v is larger than class size %v",
					structs.Type().Field(i).Name,
					klass.Type().Field(j).Name,
					field.Uint(), maxOffset)
				return vmd, nil
			}
		}
	}

	return vmd, nil
}

func newHotspotData(filename string, ef *pfelf.File) (interpreter.Data, error) {
	d := &hotspotData{}
	err := d.structPtrs.resolveSymbols(ef,
		[]string{
			"gHotSpotVMStructs",
			"gHotSpotVMStructEntryArrayStride",
			"gHotSpotVMStructEntryTypeNameOffset",
			"gHotSpotVMStructEntryFieldNameOffset",
			"gHotSpotVMStructEntryOffsetOffset",
			"gHotSpotVMStructEntryAddressOffset",
		})
	if err != nil {
		return nil, err
	}

	err = d.typePtrs.resolveSymbols(ef,
		[]string{
			"gHotSpotVMTypes",
			"gHotSpotVMTypeEntryArrayStride",
			"gHotSpotVMTypeEntryTypeNameOffset",
			"",
			"gHotSpotVMTypeEntrySizeOffset",
			"",
		})
	if err != nil {
		return nil, err
	}

	if ptr, err := locateJvmciVMStructs(ef); err == nil {
		// Everything except for the base pointer is identical.
		d.jvmciStructPtrs = d.structPtrs
		d.jvmciStructPtrs.base = ptr
		d.jvmciStructPtrs.skipBaseDref = true
	} else {
		log.Warnf("%s: unable to read JVMCI VM structs: %v", filename, err)
	}

	return d, nil
}
