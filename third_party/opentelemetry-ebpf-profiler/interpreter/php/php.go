// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package php // import "go.opentelemetry.io/ebpf-profiler/interpreter/php"

import (
	"bytes"
	"debug/elf"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"unsafe"

	log "github.com/sirupsen/logrus"

	"github.com/elastic/go-freelru"

	"go.opentelemetry.io/ebpf-profiler/interpreter"
	"go.opentelemetry.io/ebpf-profiler/libpf"
	"go.opentelemetry.io/ebpf-profiler/libpf/pfelf"
	"go.opentelemetry.io/ebpf-profiler/libpf/pfunsafe"
	"go.opentelemetry.io/ebpf-profiler/remotememory"
	"go.opentelemetry.io/ebpf-profiler/support"
)

const (
	// This is used to check if the VM mode is the default one
	// From https://github.com/php/php-src/blob/PHP-8.0/Zend/zend_vm_opcodes.h#L29
	ZEND_VM_KIND_HYBRID = (1 << 2)
)

const (
	// maxPHPRODataSize is the maximum PHP RO Data segment size to scan
	// (currently the largest seen is about 9M)
	maxPHPRODataSize = 16 * 1024 * 1024
)

var (
	// evalCodeFunctionName is a placeholder name to show that code has been evaluated
	// using eval in PHP.
	evalCodeFunctionName = libpf.Intern("<eval'd code>")

	// regex for the interpreter executable
	phpRegex = regexp.MustCompile(`.*/php(-cgi|-fpm)?[0-9.]*$|^php(-cgi|-fpm)?[0-9.]*$` +
		`|.*/libphp.*\.so$`)
	versionMatch = regexp.MustCompile(`^(\d+)\.(\d+)\.(\d+)`)

	// compiler check to make sure the needed interfaces are satisfied
	_ interpreter.Data     = &phpData{}
	_ interpreter.Instance = &phpInstance{}
)

func phpVersion(major, minor, release uint) uint {
	return major*0x10000 + minor*0x100 + release
}

type phpData struct {
	version uint

	// egAddr is the `executor_globals` symbol value which is needed by the eBPF
	// program to build php backtraces.
	egAddr libpf.Address

	// rtAddr is the `return address` for the JIT code (in Hybrid mode).
	// This is described in more detail in the PHP unwinding doc,
	// but the short description is that PHP call stacks don't always
	// store return addresses.
	rtAddr libpf.Address

	// vmStructs reflects the PHP internal class names and the offsets of named field
	vmStructs struct {
		// https://github.com/php/php-src/blob/PHP-7.4/Zend/zend_globals.h#L135
		zend_executor_globals struct {
			current_execute_data uint
		}
		// https://github.com/php/php-src/blob/PHP-7.4/Zend/zend_compile.h#L503
		zend_execute_data struct {
			opline, function  uint8
			this_type_info    uint8
			prev_execute_data uint8
		}
		// https://github.com/php/php-src/blob/PHP-7.4/Zend/zend_compile.h#L483
		zend_function struct {
			common_type, common_funcname          uint8
			op_array_filename, op_array_linestart uint
			Sizeof                                uint
		}
		// https://github.com/php/php-src/blob/PHP-7.4/Zend/zend_types.h#L235
		zend_string struct {
			val libpf.Address
		}
		// https://github.com/php/php-src/blob/PHP-7.4/Zend/zend_compile.h#L136
		zend_op struct {
			lineno uint8
		}
	}
}

func (d *phpData) String() string {
	ver := d.version
	return fmt.Sprintf("PHP %d.%d.%d", (ver>>16)&0xff, (ver>>8)&0xff, ver&0xff)
}

func (d *phpData) Attach(ebpf interpreter.EbpfHandler, pid libpf.PID, bias libpf.Address,
	rm remotememory.RemoteMemory) (interpreter.Instance, error) {
	addrToFunction, err :=
		freelru.New[libpf.Address, *phpFunction](interpreter.LruFunctionCacheSize,
			libpf.Address.Hash32)
	if err != nil {
		return nil, err
	}

	vms := &d.vmStructs
	data := support.PHPProcInfo{
		Current_execute_data: uint64(d.egAddr+bias) +
			uint64(vms.zend_executor_globals.current_execute_data),
		Jit_return_address:                  uint64(d.rtAddr + bias),
		Zend_execute_data_function:          vms.zend_execute_data.function,
		Zend_execute_data_opline:            vms.zend_execute_data.opline,
		Zend_execute_data_prev_execute_data: vms.zend_execute_data.prev_execute_data,
		Zend_execute_data_this_type_info:    vms.zend_execute_data.this_type_info,
		Zend_function_type:                  vms.zend_function.common_type,
		Zend_op_lineno:                      vms.zend_op.lineno,
	}
	if err := ebpf.UpdateProcData(libpf.PHP, pid, unsafe.Pointer(&data)); err != nil {
		return nil, err
	}

	instance := &phpInstance{
		d:              d,
		rm:             rm,
		addrToFunction: addrToFunction,
	}

	// If we failed to find the return address we need to increment
	// the value here. This happens once per interpreter instance,
	// but tracking it will help debugging later.
	if d.rtAddr == 0 && d.version >= phpVersion(8, 0, 0) {
		instance.vmRTCount.Store(1)
	}

	return instance, nil
}

func (d *phpData) Unload(_ interpreter.EbpfHandler) {
}

func versionExtract(rodata string) (uint, error) {
	matches := versionMatch.FindStringSubmatch(rodata)
	if matches == nil {
		return 0, errors.New("no valid PHP version string found")
	}

	major, _ := strconv.Atoi(matches[1])
	minor, _ := strconv.Atoi(matches[2])
	release, _ := strconv.Atoi(matches[3])
	return phpVersion(uint(major), uint(minor), uint(release)), nil
}

func determinePHPVersion(ef *pfelf.File) (uint, error) {
	// There is no ideal way to get the PHP version. This just searches
	// for a known string with the version number from .rodata.
	if ef.ROData == nil {
		return 0, errors.New("no RO data")
	}

	needle := []byte("X-Powered-By: PHP/")
	for _, segment := range ef.ROData {
		rodata, err := segment.Data(maxPHPRODataSize)
		if err != nil {
			return 0, err
		}
		idx := bytes.Index(rodata, needle)
		if idx < 0 {
			continue
		}

		idx += len(needle)
		zeroIdx := bytes.IndexByte(rodata[idx:], 0)
		if zeroIdx < 0 {
			continue
		}
		version, err := versionExtract(pfunsafe.ToString(rodata[idx : idx+zeroIdx]))
		if err != nil {
			continue
		}
		return version, nil
	}

	return 0, errors.New("no segment contained X-Powered-By")
}

func recoverExecuteExJumpLabelAddress(ef *pfelf.File) (libpf.SymbolValue, error) {
	// This function recovers the return address for JIT'd PHP code by
	// disassembling the execute_ex function. This is entirely heuristic and
	// described in some detail in the PHP8 unwinding document in the "disassembling
	// execute_ex" section. This is only useful for PHP8+

	// Zend/zend_vm_execute.h: execute_ex(zend_execute_data *ex) is the main VM
	// executor function, has been such at least since PHP7.0. This is guaranteed
	// to be the vm executor function in PHP JIT'd code, since the JIT is (currently)
	// inoperable with overridden execute_ex's

	// The address we care about varies from being 47 bytes in to about 107 bytes in,
	// so we'll cap at 128 bytes. This might need to be adjusted up in future.
	sym, code, err := ef.SymbolData("execute_ex", 128)
	if err != nil {
		return libpf.SymbolValueInvalid,
			fmt.Errorf("unable to read 'execute_ex': %w", err)
	}
	var returnAddress libpf.SymbolValue
	switch ef.Machine {
	case elf.EM_AARCH64:
		returnAddress, err = retrieveExecuteExJumpLabelAddressARM(code, sym.Address)
	case elf.EM_X86_64:
		returnAddress, err = retrieveExecuteExJumpLabelAddressX86(code, sym.Address)
	default:
		return returnAddress, fmt.Errorf("unsupported architecture: %s", ef.Machine)
	}
	if err != nil {
		return libpf.SymbolValueInvalid,
			fmt.Errorf("reading the return address from execute_ex failed (%w)",
				err)
	}

	return returnAddress, nil
}

func determineVMKind(ef *pfelf.File) (uint, error) {
	// This function recovers the PHP VM mode from the PHP binary
	// This is a compile-time configuration option that configures
	// how the PHP VM calls functions. This is only useful for PHP8+

	// This is a publicly exposed function in PHP that returns the VM type
	// This has been implemented in PHP since at least 7.2

	// We should only need around 32 bytes here, since this function should be
	// really short (e.g a mov and a ret).
	_, code, err := ef.SymbolData("zend_vm_kind", 64)
	if err != nil {
		return 0, fmt.Errorf("unable to read 'zend_vm_kind': %w", err)
	}
	var vmKind uint
	switch ef.Machine {
	case elf.EM_AARCH64:
		vmKind, err = retrieveZendVMKindARM(code)
	case elf.EM_X86_64:
		vmKind, err = retrieveZendVMKindX86(code)
	default:
		return 0, fmt.Errorf("unsupported architecture: %s", ef.Machine)
	}
	if err != nil {
		return 0, fmt.Errorf("an error occurred decoding zend_vm_kind: %w", err)
	}
	return vmKind, nil
}

func Loader(ebpf interpreter.EbpfHandler, info *interpreter.LoaderInfo) (interpreter.Data, error) {
	if !phpRegex.MatchString(info.FileName()) {
		return nil, nil
	}

	ef, err := info.GetELF()
	if err != nil {
		return nil, err
	}

	version, err := determinePHPVersion(ef)
	if err != nil {
		return nil, err
	}

	// Only tested on PHP7.3-PHP8.3. Other similar versions probably only require
	// tweaking the offsets.
	var minVer, maxVer = phpVersion(7, 3, 0), phpVersion(8, 4, 0)
	if version < minVer || version >= maxVer {
		return nil, fmt.Errorf("PHP version %d.%d.%d (need >= %d.%d and < %d.%d)",
			(version>>16)&0xff, (version>>8)&0xff, version&0xff,
			(minVer>>16)&0xff, (minVer>>8)&0xff,
			(maxVer>>16)&0xff, (maxVer>>8)&0xff)
	}

	egAddr, err := ef.LookupSymbolAddress("executor_globals")
	if err != nil {
		return nil, fmt.Errorf("PHP %x: executor_globals not found: %v", version, err)
	}

	// Zend/zend_vm_execute.h: execute_ex(zend_execute_data *ex) is the main VM
	// executor function, has been such at least since PHP7.0.
	interpRanges, err := info.GetSymbolAsRanges("execute_ex")
	if err != nil {
		return nil, err
	}

	// If the version is PHP8+ we need to be able to
	// potentially unwind JIT code. For now we need to recover
	// the return address and check that hybrid mode is used. This is the
	// default mode (there are others but they should be rarely used in production)
	// Note that if there is an error in the block below then unwinding will produce
	// incomplete stack unwindings if the JIT compiler is used.
	rtAddr := libpf.SymbolValueInvalid
	if version >= phpVersion(8, 0, 0) {
		var vmKind uint
		vmKind, err = determineVMKind(ef)
		if err != nil {
			log.Debugf("PHP version %x: an error occurred while determining "+
				"the VM kind (%v)",
				version, err)
		} else if vmKind == ZEND_VM_KIND_HYBRID {
			rtAddr, err = recoverExecuteExJumpLabelAddress(ef)
			if err != nil {
				log.Debugf("PHP version %x: an error occurred while determining "+
					"the return address for execute_ex: (%v)", version, err)
			}
		}
	}
	pid := &phpData{
		version: version,
		egAddr:  libpf.Address(egAddr),
		rtAddr:  libpf.Address(rtAddr),
	}

	// PHP does not provide introspection data, hard code the struct field
	// offsets based on detected version. Some values can be fairly easily
	// calculated from the struct definitions, but some are looked up by
	// using gdb and getting the field offset directly from debug data.
	vms := &pid.vmStructs
	vms.zend_executor_globals.current_execute_data = 488
	vms.zend_execute_data.opline = 0
	vms.zend_execute_data.function = 24
	vms.zend_execute_data.this_type_info = 40
	vms.zend_execute_data.prev_execute_data = 48
	vms.zend_function.common_type = 0
	vms.zend_function.common_funcname = 8
	vms.zend_function.op_array_filename = 128
	vms.zend_function.op_array_linestart = 136
	// Note: the sizeof here isn't actually the sizeof the
	// zend_function object. This is set to 168
	// primarily for efficiency reasons, since we
	// need at most 168 bytes.
	vms.zend_function.Sizeof = 168
	vms.zend_string.val = 24
	vms.zend_op.lineno = 24
	switch {
	case version >= phpVersion(8, 3, 0):
		vms.zend_function.op_array_filename = 144
		vms.zend_function.op_array_linestart = 152
	case version >= phpVersion(8, 2, 0):
		vms.zend_function.op_array_filename = 152
		vms.zend_function.op_array_linestart = 160
	case version >= phpVersion(8, 0, 0):
		vms.zend_function.op_array_filename = 144
		vms.zend_function.op_array_linestart = 152
	case version >= phpVersion(7, 4, 0):
		vms.zend_function.op_array_filename = 136
		vms.zend_function.op_array_linestart = 144
	}

	if err = ebpf.UpdateInterpreterOffsets(support.ProgUnwindPHP,
		info.FileID(), interpRanges); err != nil {
		return nil, err
	}

	return pid, nil
}
