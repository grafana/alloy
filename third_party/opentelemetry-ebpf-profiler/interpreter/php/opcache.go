// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package php // import "go.opentelemetry.io/ebpf-profiler/interpreter/php"

//nolint:lll
// PHP8+ JIT compiler unwinder.
// This file contains the code necessary for unwinding PHP code that has been JIT compiled.
//
// TL;DR: This file exists just to provide the PHP unwinder with the right PIDPages for JIT'd code.
// Everything else in interpreterphp works as expected for unwinding this code.
//
// It turns out that the PHP JIT compiler is a little bit strange compared to other JIT compilers
// (e.g V8) because of the unique limitations of the PHP compiler (or, rather, over 20 years of
// organic code growth).
//
// If you want to understand how the PHP JIT compiler actually works, there's no substitute
// for reading the PHP source code documents. In particular, these are useful:
//
// 1) https://github.com/php/php-src/blob/PHP-8.0/ext/opcache/jit/zend_jit.h
// 2) https://github.com/php/php-src/blob/PHP-8.0/ext/opcache/jit/zend_jit.c (if you have a while)
//
// It might also be useful to know how Zend Extensions work (or, at least be aware of their existence)
// It turns out the way these have been structured implies almost everything you need to know about
// why the JIT compiler is the way it is. This is a useful resource for understanding them
// https://www.phpinternalsbook.com/php7/extensions_design/zend_extensions.html
//
// The PHP JIT compiler uses dynasm for the actual JITing portion of the code. The informal
// tutorial is really good, you should read it (https://corsix.github.io/dynasm-doc/tutorial.html)
// You don't need to understand how Dynasm actually works for understanding this code, but it
// still might be useful for other projects.
//
// Before we begin it's illustrative to understand how PHP works internally.
// PHP belongs to the class of interpreted languages that use bytecode: each PHP function is
// decomposed into a sequence of bytecode instructions that are then executed by the PHP interpreter.
// These instructions are known as zend_ops in the Zend compiler, and their internal structure
// looks like this:
//  struct _zend_op {
//     const void *handler;
//     znode_op op1;
//     znode_op op2;
//     znode_op result;
//     ....
//  };
//
// Here the "handler" member is a pointer to some function that is executed when the
// zend_op is evaluated. Typically this is a PHP function of some kind that has already been
// pre-built.  You can imagine this as a function pointer to some function that
// accepts two arguments and produces a singular result. Note that znode_ops can refer to other _zend_ops,
// which allows you to build a AST.
// A good resource on this (with far more detail, if you need it) is
// https://www.npopov.com/2017/04/14/PHP-7-Virtual-machine.html
//
// The way that the JIT compiler works is that it replaces the handler pointer with the address
// of some JIT'd code.
// In other words, there's some code somewhere that does something like this:
//              my_op->handler = &some_function;
// (Or, exactly, this: https://github.com/php/php-src/blob/PHP-8.0/ext/opcache/jit/zend_jit.c#L427)
// This means that when the zend_op is evaluated native code is used
// rather than the PHP function, which enables the code to be much faster.
//
// The implication of this is that the PHP unwinder doesn't actually need to be changed at all (beyond a few offsets)
// since the executor_globals is still the primary point of execution[1]. This means that all we have to do
// is tell the eBPF code to unwind PHP when JIT'd code is encountered.
//
// However, getting the JIT memory regions inside the base PHP interpreter is difficult.
// It turns out that PHP's JIT compiler is a bit strange: the memory for the JIT'd code lives inside a
// Zend extension called the OPCache.
//
// As justification: older versions of PHP (e.g before PHP 5.5) had a problem: each time a script was executed
// the script needed to be parsed into opcodes, compiled on the virtual machine and then
// executed. This is rather inefficient for frequently called scripts: so, PHP 5.5 introduced
// the OPcache, which caches frequently used scripts and the corresponding opcodes.
// It turns out that the makers of the Zend engine reasoned that if you wanted the JIT you'd also want the Opcache.
//
// It's natural to ask where the Opcache lives in memory. Since PHP supports
// both thread and process-level parallelism, this memory needs to be shared across all PHP
// processes. This means that the Opcache doesn't live in any single processes memory; it's actually
// allocated in shared memory.
//
// The implication of this are:
// a) JIT'd code lives in shared memory, which means that all of the process-local work that the host-agent normally does doesn't really apply for PHP.
// b) The PHP JIT doesn't even live in the same shared object as the PHP interpreter, so we can't
//    find the JIT information from the PHP interpreter.
// c) Even if we could, PHP hides symbols by default and so recovering the relevant information isn't easy in this form[2].
//
// Note that we also can't use the approach used in the V8 interpreter
// because the JIT'd PHP code doesn't ever get loaded as an anonymous mapping (the JIT region is just
// marked as executable and it's never loaded into the executable directly).
// In a sense this is more like how the Hotspot Interpreter works.
//
// The solution for this problem we use here is to resolve the OPcache mapping for the PHP process.
// The reasons for this are:
// a) The OPcache contains symbols for the externally-exposed JIT functions.
// b) At least one of those functions sets a both a pointer to the JIT memory and a variable
//    that contains the size of the buffer.
// c) The OPcache is the shared object that actually allocates the memory: when the OPcache extension
//    is initialized the memory is allocated. This also just makes everything a bit neater.
//
// This means that we can inform the eBPF code that the PHP unwinder should be used whenever JIT'd PHP code is encountered.
//
// The design of this interpreter is therefore as follows: we don't do _any_ PHP unwinding in
// this interpreter at all. This interpreter is solely meant to allow the PHP unwinder to be triggered
// when appropriate. This means that the interpreter is really basic compared to all of the other
// interpreters.
//
// Footnotes:
// (1) In different modes there are other approaches that you can use to do this sort of unwinding. For example, in Debug mode the Zend compiler stores
//     information about each JIT'd frames in a jit_globals structure. This is probably really useful if the client is running in Debug mode (or if they've
//     compiled PHP with HAVE_GDB enabled or similar) but this isn't guaranteed to work across all deployments (whereas this version should).
// (2) The original version (i.e pre-PR) code tried to do this. You can (in theory) walk the module registry that PHP provides to find the JIT info at runtime
//     and then you can do this all inside interpreterphp. However, this turned out to be really complicated, brittle and less efficient than this approach (there were
//     far more memory reads from the particular process during the initial loading than with this approach).
// (3) Note that this code should also work with PHP's thread-safe resource management mechanism. Since the JIT buffer is shared across all processes anyway clients who
//     use the TSRM shouldn't encounter any issues here. There are other uncommon ways to build PHP, but these also shouldn't affect how this code works.

import (
	"debug/elf"
	"encoding/binary"
	"errors"
	"fmt"
	"regexp"

	log "github.com/sirupsen/logrus"

	"go.opentelemetry.io/ebpf-profiler/interpreter"
	"go.opentelemetry.io/ebpf-profiler/libpf"
	"go.opentelemetry.io/ebpf-profiler/libpf/pfelf"
	"go.opentelemetry.io/ebpf-profiler/lpm"
	"go.opentelemetry.io/ebpf-profiler/process"
	"go.opentelemetry.io/ebpf-profiler/remotememory"
	"go.opentelemetry.io/ebpf-profiler/reporter"
	"go.opentelemetry.io/ebpf-profiler/support"
	"go.opentelemetry.io/ebpf-profiler/util"
)

var (
	// Regex from the opcache.
	opcacheRegex = regexp.MustCompile(`^(?:.*/)?opcache\.so$`)
	// Make sure that the needed interfaces are satisfied
	_ interpreter.Data     = &opcacheData{}
	_ interpreter.Instance = &opcacheInstance{}
)

type opcacheData struct {
	version uint

	// dasmBuf is the address of the shared memory that is used for the JIT'd code.
	// This is defined here:
	// https://github.com/php/php-src/blob/PHP-8.0/ext/opcache/jit/zend_jit.c#L103
	dasmBufPtr libpf.Address

	// dasmSize is the size of the JIT buffer.
	// This is defined here:
	// https://github.com/php/php-src/blob/PHP-8.0/ext/opcache/jit/zend_jit.c#L107
	dasmSizePtr libpf.Address
}

type opcacheInstance struct {
	interpreter.InstanceStubs

	// d is the interpreter data from opcache.so (shared between processes)
	d *opcacheData

	// rm is used to access the remote process memory
	rm remotememory.RemoteMemory

	// bias is the load bias
	bias libpf.Address

	// prefixes is the list of LPM prefixes added to ebpf maps (to be cleaned up)
	prefixes []lpm.Prefix
}

func (i *opcacheInstance) Detach(ebpf interpreter.EbpfHandler, pid libpf.PID) error {
	// Here we just remove the entries relating to the mappings for the
	// JIT's memory
	var err error

	for _, prefix := range i.prefixes {
		if err2 := ebpf.DeletePidInterpreterMapping(pid, prefix); err2 != nil {
			err = errors.Join(err, fmt.Errorf("failed to remove page 0x%x/%d: %w",
				prefix.Key, prefix.Length, err2))
		}
	}

	if err != nil {
		return fmt.Errorf("failed to detach opcacheInstance from PID %d: %w",
			pid, err)
	}

	return nil
}

func (i *opcacheInstance) SynchronizeMappings(ebpf interpreter.EbpfHandler,
	_ reporter.ExecutableReporter, pr process.Process, _ []process.Mapping) error {
	if i.prefixes != nil {
		// Already attached
		return nil
	}

	dasmBufVal := make([]byte, 8)
	dasmSizeVal := make([]byte, 8)
	if err := i.rm.Read(i.d.dasmBufPtr+i.bias, dasmBufVal); err != nil {
		return nil
	}
	if err := i.rm.Read(i.d.dasmSizePtr+i.bias, dasmSizeVal); err != nil {
		return nil
	}

	dasmBuf := binary.LittleEndian.Uint64(dasmBufVal)
	dasmSize := binary.LittleEndian.Uint64(dasmBufVal)
	if dasmBuf == 0 || dasmSize == 0 {
		// This is the normal path if JIT is not enabled, or we try to
		// attach before JIT engine is initialized.
		return nil
	}

	prefixes, err := lpm.CalculatePrefixList(dasmBuf, dasmBuf+dasmSize)
	if err != nil {
		log.Debugf("Producing prefixes failed: %v", err)
		return err
	}

	pid := pr.PID()
	i.prefixes = prefixes
	for _, prefix := range prefixes {
		err = ebpf.UpdatePidInterpreterMapping(pid, prefix, support.ProgUnwindPHP, 0, 0)
		if err != nil {
			return err
		}
	}
	return nil
}

func (d *opcacheData) String() string {
	ver := d.version
	return fmt.Sprintf("Opcache %d.%d.%d", (ver>>16)&0xff, (ver>>8)&0xff, ver&0xff)
}

func (d *opcacheData) Attach(_ interpreter.EbpfHandler, _ libpf.PID, bias libpf.Address,
	rm remotememory.RemoteMemory) (interpreter.Instance, error) {
	return &opcacheInstance{
		d:    d,
		rm:   rm,
		bias: bias,
	}, nil
}

func (d *opcacheData) Unload(_ interpreter.EbpfHandler) {
}

func determineOPCacheVersion(ef *pfelf.File) (uint, error) {
	// In contrast to interpreterphp, the opcache actually contains
	// a really straightforward way to recover the version. As the opcache
	// is a Zend extension, it has to provide a version, which just so
	// happens to be the PHP version.
	//
	// The way this function works is as follows.
	// Each zend_extension in PHP looks something like this:
	//
	// Courtesy of https://github.com/php/php-src/blob/PHP-8.0/Zend/zend_extensions.h#L77,
	// but the structure's layout hasn't changed in 20+ years
	//
	// struct _zend_extension {
	//   char *name;
	//   char *version;
	//   ...
	// };
	//
	// Now normally this could be anything: anyone can write a zend extension.
	// However, for the opcache in particular this is exactly the PHP version:
	//
	// https://github.com/php/php-src/blob/PHP-8.0/ext/opcache/ZendAccelerator.c#L4994
	//
	// ZEND_EXT_API zend_extension zend_extension_entry = {
	//          ACCELERATOR_PRODUCT_NAME,
	//          PHP_VERSION,
	//            ...
	// };
	//
	// Since the version is the PHP_VERSION, we can just recover the version by reading
	// the second pointer in the struct and parsing that. This has been the case since
	// PHP 7.0, which is good enough.

	moduleExtension, err := ef.LookupSymbolAddress("zend_extension_entry")
	if err != nil {
		return 0, fmt.Errorf("could not find zend_extension_entry: %w", err)
	}

	// The version string is the second pointer of this structure
	rm := ef.GetRemoteMemory()
	versionString := rm.StringPtr(libpf.Address(moduleExtension + 8))
	if versionString == "" || !util.IsValidString(versionString) {
		return 0, fmt.Errorf("extension entry PHP version invalid at 0x%x",
			moduleExtension)
	}

	// We should now have a string that contains the exact right version.
	return versionExtract(versionString)
}

// getOpcacheJITInfo retrieves the starting address and the size of the JIT buffer.
// If these cannot be found then (libpf.SymbolValueInvalid, 0, error)
// will be returned.
func getOpcacheJITInfo(ef *pfelf.File) (dasmBuf, dasmSize libpf.Address, err error) {
	// This function works by disassembling a particular exported function and
	// using that to recover the relevant information.
	// The steps are as follows:
	// a) Disassemble zend_jit_unprotect.
	// b) Recover the address of dasm_buf and the size of the buffer.
	// Note: zend_jit_unprotect was chosen because it immediately calls mprotect with
	// dasm_buf as the first parameter, which should be in a register for both x86-64
	// and ARM64.

	// We should only need 64 bytes, since this should be early in the instruction sequence.
	sym, code, err := ef.SymbolData("zend_jit_unprotect", 64)
	if err != nil {
		return 0, 0, fmt.Errorf("unable to read 'zend_jit_unprotect': %w", err)
	}
	var (
		dasmBufPtr  libpf.SymbolValue
		dasmSizePtr libpf.SymbolValue
	)
	switch ef.Machine {
	case elf.EM_AARCH64:
		dasmBufPtr, dasmSizePtr, err = retrieveJITBufferPtrARM(code, sym.Address)
	case elf.EM_X86_64:
		dasmBufPtr, dasmSizePtr, err = retrieveJITBufferPtrx86(code, sym.Address)
	default:
		return 0, 0, fmt.Errorf("unsupported machine type: %s", ef.Machine)
	}
	if err != nil {
		return 0, 0, fmt.Errorf("failed to extract DASM pointers: %w", err)
	}
	if dasmBufPtr == libpf.SymbolValueInvalid || dasmBufPtr%4 != 0 {
		return 0, 0, fmt.Errorf("dasmBufPtr %#x is invalid", dasmBufPtr)
	}
	if dasmSizePtr == libpf.SymbolValueInvalid || dasmSizePtr%4 != 0 {
		return 0, 0, fmt.Errorf("bad dasmSizePtr %#x is invalid", dasmSizePtr)
	}
	return libpf.Address(dasmBufPtr), libpf.Address(dasmSizePtr), nil
}

func OpcacheLoader(_ interpreter.EbpfHandler, info *interpreter.LoaderInfo) (
	interpreter.Data, error) {
	if !opcacheRegex.MatchString(info.FileName()) {
		return nil, nil
	}

	ef, err := info.GetELF()
	if err != nil {
		return nil, fmt.Errorf("could not get ELF: %w", err)
	}

	// Determine PHP version first
	version, err := determineOPCacheVersion(ef)
	if err != nil {
		return nil, err
	}

	// Expect PHP 8+ for proper JIT support
	if version < phpVersion(8, 0, 0) {
		return nil, nil
	}

	// Extract location from where to read dasm buffer
	dasmBufPtr, dasmSizePtr, err := getOpcacheJITInfo(ef)
	if err != nil {
		return nil, err
	}

	// We only load the JIT buffer address in Attach: this is because
	// we might need to spin on the buffer being available.
	pid := &opcacheData{
		version:     version,
		dasmBufPtr:  dasmBufPtr,
		dasmSizePtr: dasmSizePtr,
	}

	return pid, nil
}
