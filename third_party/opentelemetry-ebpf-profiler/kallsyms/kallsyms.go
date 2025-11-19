// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package kallsyms provides functionality for reading /proc/kallsyms
// and using it to symbolize kernel addresses.
package kallsyms // import "go.opentelemetry.io/ebpf-profiler/kallsyms"

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/mdlayher/kobject"
	log "github.com/sirupsen/logrus"

	"go.opentelemetry.io/ebpf-profiler/libpf"
	"go.opentelemetry.io/ebpf-profiler/libpf/pfelf"
	"go.opentelemetry.io/ebpf-profiler/libpf/pfunsafe"
	"go.opentelemetry.io/ebpf-profiler/stringutil"
)

// Kernel is the internal name for "module" containing the built-in symbols
const Kernel = "vmlinux"

// pointerBits is the number of bits for pointer. Used to validate data
// from the kernel kallsyms file.
const pointerBits = int(unsafe.Sizeof(libpf.Address(0)) * 8)

// sysModule is the sysfs path for module metadata
const sysModule = "/sys/module"

var ErrSymbolPermissions = errors.New("unable to read kallsyms addresses - check capabilities")

var ErrNoModule = errors.New("module not found")

var ErrNoSymbol = errors.New("symbol not found")

var ErrModuleStub = errors.New("symbols are not available yet - retry later")

// symbol is the per-symbol structure. The size should be minimal as
// a typical installation has 100k-200k kernel symbols.
type symbol struct {
	// offset is the symbol offset from the Module start address
	offset uint32
	// index is the offset to the symbol name within the Module names slice
	index uint32
}

// Module contains symbols and metadata for one kernel module.
type Module struct {
	start libpf.Address
	end   libpf.Address
	mtime int64
	stub  bool

	mappingFile libpf.FrameMappingFile

	names   []byte
	symbols []symbol
}

// Symbolizer provides the main API for reading, updating and querying
// the kernel symbols.
type Symbolizer struct {
	modules atomic.Value

	reloadModules chan bool
}

// NewSymbolizer creates and returns a new kallsyms symbolizer and loads
// the initial 'kallsymbols'.
func NewSymbolizer() (*Symbolizer, error) {
	s := &Symbolizer{
		reloadModules: make(chan bool, 1),
	}
	if err := s.loadKallsyms(); err != nil {
		return nil, err
	}
	return s, nil
}

// addName appends the 'name' to the module's string slice, and returns
// an index suitable for storing in the `symbol` struct.
func (m *Module) addName(name string) uint32 {
	index := len(m.names)
	// Cap the length to 255 bytes so it fits a byte. Longest seen
	// symbol so far is 83 bytes.
	l := min(len(name), 255)
	m.names = append(m.names, byte(l))
	m.names = append(m.names, unsafe.Slice(unsafe.StringData(name), l)...)
	return uint32(index)
}

// setStub makes this module a stub entry for given module name.
func (m *Module) setStub(name string) {
	m.names = make([]byte, 0, len(name))
	m.addName(name)
	m.stub = true
}

// bytesAt recovers a []byte representation of the string at `index`
// received from previous `addName` call.
func (m *Module) bytesAt(index uint32) []byte {
	i := int(index)
	l := int(m.names[i])
	return m.names[i+1 : i+1+l]
}

// stringAt recovers the string at `index` received from previous `addName` call.
func (m *Module) stringAt(index uint32) string {
	return pfunsafe.ToString(m.bytesAt(index))
}

// parseSysfsUint reads a kernel module specific attribute from sysfs.
func parseSysfsUint(mod, knob string) (uint64, error) {
	text, err := os.ReadFile(path.Join(sysModule, mod, knob))
	if err != nil {
		return 0, err
	}
	return strconv.ParseUint(strings.Trim(pfunsafe.ToString(text), "\n"), 0, pointerBits)
}

// loadModuleMetadata is the function to load module bounds and fileID data.
// Overridable for the test suite. Returns true if the metadata was loaded
// successfully.
var loadModuleMetadata = func(m *Module, name string, oldMtime int64) bool {
	if name == "bpf" {
		// Kernel reports the BPF JIT symbols as part of 'bpf' module.
		// There is no metadata available.
		return true
	}

	// Determine notes location and module size
	notesFile := "/sys/kernel/notes"
	if name != Kernel {
		info, err := os.Stat(path.Join(sysModule, name))
		if err != nil {
			return false
		}
		m.mtime = info.ModTime().UnixMilli()
		if m.mtime == oldMtime {
			return false
		}

		notesFile = path.Join(sysModule, name, "notes/.note.gnu.build-id")
		addr, err := parseSysfsUint(name, "sections/.text")
		if err != nil {
			return false
		}
		size, err := parseSysfsUint(name, "coresize")
		if err != nil {
			return false
		}
		m.start = libpf.Address(addr)
		m.end = m.start + libpf.Address(size)
	} else {
		// No need to reload kernel symbols
		if m.mtime == 1 {
			return false
		}
		m.mtime = 1
	}

	// Require at least 16 bytes of BuildID to ensure there is enough entropy for a FileID.
	// 16 bytes could happen when --build-id=md5 is passed to `ld`. This would imply a custom
	// kernel.
	fileID := libpf.UnknownKernelFileID
	buildID, err := pfelf.GetBuildIDFromNotesFile(notesFile)
	if err == nil && len(buildID) >= 16 {
		fileID = libpf.FileIDFromKernelBuildID(buildID)
	}
	m.mappingFile = libpf.NewFrameMappingFile(libpf.FrameMappingFileData{
		FileID:     fileID,
		FileName:   libpf.Intern(name),
		GnuBuildID: buildID,
	})
	return true
}

// finish will finalize the Module. The 'symbols' slice is sorted, and
// a fallback fileID is synthesized if buildID is not available.
func (m *Module) finish() {
	if m.end == 0 || m.end == ^libpf.Address(0) {
		// Synthesize the end address at last symbol rounded up to page size
		// because it could not be reliably determined.
		lastSymbol := m.start + libpf.Address(m.symbols[len(m.symbols)-1].offset)
		m.end = (lastSymbol + 4095) & ^libpf.Address(4095)
	}

	sort.Slice(m.symbols, func(i, j int) bool {
		return m.symbols[i].offset >= m.symbols[j].offset
	})
}

func (m *Module) Name() string {
	return m.stringAt(0)
}

func (m *Module) Start() libpf.Address {
	return m.start
}

func (m *Module) End() libpf.Address {
	return m.end
}

func (m *Module) MappingFile() libpf.FrameMappingFile {
	return m.mappingFile
}

// LookupSymbolByAddress resolves the `pc` address to the function and offset from it.
// On error, an empty string with zero offset is returned.
func (m *Module) LookupSymbolByAddress(pc libpf.Address) (funcName string, offset uint, err error) {
	if m.stub {
		return "", 0, ErrModuleStub
	}
	pcOffs := uint32(pc - m.start)
	symIdx := sort.Search(len(m.symbols), func(i int) bool {
		return pcOffs >= m.symbols[i].offset
	})
	if symIdx >= len(m.symbols) {
		return "", 0, ErrNoSymbol
	}
	sym := &m.symbols[symIdx]
	symName := m.stringAt(sym.index)
	return symName, uint(pcOffs - sym.offset), nil
}

// LookupSymbol finds a symbol with 'name' from the Module.
func (m *Module) LookupSymbol(name string) (libpf.Address, error) {
	for _, sym := range m.symbols {
		if m.stringAt(sym.index) == name {
			return m.start + libpf.Address(sym.offset), nil
		}
	}
	return 0, ErrNoSymbol
}

// LookupSymbolsByPrefix finds all symbols with the given prefix in from the Module.
func (m *Module) LookupSymbolsByPrefix(prefix string) []*libpf.Symbol {
	res := make([]*libpf.Symbol, 0, 8)
	for _, sym := range m.symbols {
		symName := m.stringAt(sym.index)
		if strings.HasPrefix(symName, prefix) {
			symAddr := m.start + libpf.Address(sym.offset)
			res = append(res, &libpf.Symbol{
				Name:    libpf.SymbolName(symName),
				Address: libpf.SymbolValue(symAddr),
			})
		}
	}
	return res
}

// updateSymbolsFrom parses /proc/kallsyms format data from the reader 'r'.
// If possible the data from previous reads is re-used to avoid allocations.
// The Symbolizer internal state is updated only if the input data is parsed
// successfully.
func (s *Symbolizer) updateSymbolsFrom(r io.Reader) error {
	var mod *Module
	var curName string
	var syms []symbol
	var names []byte

	noSymbols := true
	modules, _ := s.modules.Load().([]Module)

	// The kallsyms symbol order is the following:
	// 1. kernel symbols (from compressed kallsyms)
	// 2. kernel arch symbols (if any)
	// 3. module symbols (grouped by module from all loaded modules)
	// 4. module symbols ftrace cloned from __init section
	//    (all __init symbols ftrace traced during module load)
	// 5. bpf module symbols (dynamically generated from JITted bpf programs)
	//
	// We load the per-module symbols from group #3 in one go. We also generally
	// do not care about the symbols in group #4 as they are only the __init
	// symbols after they have been freed. Trying to use these symbols is
	// problematic:
	// 1. the symbol data is normally not present at all
	// 2. they are used during init only (getting traces with them is unlikely)
	// 3. after the __init data is freed, the same VMA range can be reused for
	//    another newly loaded module. deciding afterwards if it was the now
	//    released __init symbol or the newly loaded module code is non-trivial.
	// 4. loading these symbols means we would have potentially overlapping symbols.
	//
	// For the above reasons, it is better to just ignore these ftrace cloned
	// __init symbols. This is done with the 'seen' set to avoid loading symbols
	// for a module if has been already processed.
	seen := make(libpf.Set[string])

	// Allocate buffers which should be able to hold the symbol data
	// from the vmlinux main image (or large modules on reloads) without
	// resizing based on normal distribution kernel. These are later
	// cloned to the exact size needed, so these are stack allocated.

	// The modules (typical systems have 200-300)
	mods := make([]Module, 0, 400)
	if len(modules) == 0 {
		// - 2.5MB for symbol names
		// - 100k symbols
		names = make([]byte, 0, 3*1024*1024)
		syms = make([]symbol, 0, 128*1024)
	} else {
		// - 0.5MB for symbol names (e.g. i915 needs 400k)
		// - 64k symbols (e.g. i915 has 12k symbols)
		names = make([]byte, 0, 512*1024)
		syms = make([]symbol, 0, 64*1024)

		// Copy the static symbols here. The kallsyms often starts
		// with symbols not within kernel .text, and the logic below
		// would not correctly detect already seen kernel symbols.
		for _, mod := range modules {
			if mod.Name() == Kernel {
				mods = append(mods, mod)
				seen[Kernel] = libpf.Void{}
				break
			}
		}
	}

	for scanner := bufio.NewScanner(r); scanner.Scan(); {
		// Avoid heap allocation by not using scanner.Text().
		// NOTE: The underlying bytes will change with the next call to scanner.Scan(),
		// so make sure to not keep any references after the end of the loop iteration.
		line := pfunsafe.ToString(scanner.Bytes())

		// Avoid heap allocations here - do not use strings.FieldsN()
		var fields [4]string
		nFields := stringutil.FieldsN(line, fields[:])
		if nFields < 3 {
			return fmt.Errorf("unexpected line in kallsyms: '%s'", line)
		}

		// Skip non-text symbols, see 'man nm'.
		// Special case for 'etext', which can be of type `D` (data) in some kernels.
		if strings.IndexByte("TtVvWw", fields[1][0]) == -1 && fields[2] != "_etext" {
			continue
		}

		address, err := strconv.ParseUint(fields[0], 16, pointerBits)
		if err != nil {
			return fmt.Errorf("failed to parse address value: '%s'", fields[0])
		}
		if address != 0 {
			noSymbols = false
		}

		moduleName := Kernel
		if fields[3] != "" {
			moduleName = fields[3]
			if moduleName[0] != '[' && moduleName[len(moduleName)-1] != ']' {
				return fmt.Errorf("failed to parse module name: '%s'", moduleName)
			}
			moduleName = moduleName[1 : len(moduleName)-1]
		}

		if curName != moduleName {
			if curName == Kernel && noSymbols {
				return ErrSymbolPermissions
			}
			if mod != nil && len(mod.symbols) > 0 {
				// Update the working buffers from potentially reallocated
				// slices to avoid continuous reallocations.
				names = mod.names[0:0]
				syms = mod.symbols[0:0]
				// Clone a copy of the data to the module so that it does not
				// overlap with the working buffer, and is sized exactly the
				// needed size.
				mod.names = bytes.Clone(mod.names)
				mod.symbols = slices.Clone(mod.symbols)
				mod.finish()
				// Update seen map with the cloned module name string so
				// it does not get overwritten later on.
				seen[mod.Name()] = libpf.Void{}
			}
			mod = nil

			if _, ok := seen[moduleName]; !ok {
				var oldMod *Module
				var oldMtime int64
				newMod := Module{
					end:     ^libpf.Address(0),
					symbols: syms[0:0],
					names:   names[0:0],
				}
				if moduleName != "bpf" {
					oldMod, _ = getModuleByAddress(modules, libpf.Address(address))
					if oldMod != nil && !oldMod.stub && oldMod.Name() == moduleName {
						oldMtime = oldMod.mtime
					} else {
						oldMod = nil
					}
				}
				if loadModuleMetadata(&newMod, moduleName, oldMtime) {
					// Module metadata was updated. Parse this module symbols.
					mods = append(mods, newMod)
					mod = &mods[len(mods)-1]
					mod.addName(moduleName)
					curName = mod.Name()
				} else if oldMod != nil {
					// Reuse the existing module data if any.
					mods = append(mods, *oldMod)
					curName = oldMod.Name()
				}
			}
		}

		if mod == nil {
			continue
		}

		switch fields[2] {
		case "_stext", "_text":
			if mod.start == 0 {
				mod.start = libpf.Address(address)
			}
		case "_etext":
			if mod.end == ^libpf.Address(0) {
				mod.end = libpf.Address(address)
			}
		case "_sinittext", "_einittext":
		default:
			if mod.start == 0 {
				mod.start = libpf.Address(address)
			}
			if addr := libpf.Address(address); addr >= mod.start && addr < mod.end {
				// Add symbol to the module symbols
				mod.symbols = append(mod.symbols, symbol{
					offset: uint32(addr - mod.start),
					index:  mod.addName(fields[2]),
				})
			}
		}
	}
	if mod != nil {
		mod.finish()
	}
	if noSymbols {
		return ErrSymbolPermissions
	}

	sort.Slice(mods, func(i, j int) bool {
		return mods[i].start >= mods[j].start
	})
	// Heap allocate the exact amount needed. This also makes the initial
	// buffer stack allocated.
	s.modules.Store(slices.Clone(mods))
	return nil
}

// loadKallsyms will reload kernel symbols. This function can run concurrently with
// module and symbol lookups. The reload result is visible atomically after success.
func (s *Symbolizer) loadKallsyms() error {
	file, err := os.Open("/proc/kallsyms")
	if err != nil {
		return fmt.Errorf("unable to open kallsyms: %v", err)
	}
	defer file.Close()

	return s.updateSymbolsFrom(file)
}

var nonsyfsModules = libpf.Set[string]{
	Kernel: libpf.Void{},
	"bpf":  libpf.Void{},
}

// loadModules will reload module metadata.
func (s *Symbolizer) loadModules() (bool, error) {
	dir, err := os.Open(sysModule)
	if err != nil {
		return false, err
	}
	defer dir.Close()

	needReloadSymbols := false
	modules, _ := s.modules.Load().([]Module)
	mods := make([]Module, 0, 400)

	// Copy the modules not present in sysfs
	for _, mod := range modules {
		if _, ok := nonsyfsModules[mod.Name()]; ok {
			mods = append(mods, mod)
		}
	}

	// Scan sysfs for current module listing and its metadata
	for {
		dirEntries, err := dir.ReadDir(64)
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return false, err
		}
		for _, dirEnt := range dirEntries {
			if !dirEnt.IsDir() {
				continue
			}

			moduleName := dirEnt.Name()
			curMod := Module{}
			if !loadModuleMetadata(&curMod, moduleName, 0) {
				// sysfs contains directories also for statically built
				// kernel modules. Ignore these.
				continue
			}

			oldMod, _ := getModuleByAddress(modules, curMod.start)
			if oldMod != nil && oldMod.Name() == moduleName && oldMod.mtime == curMod.mtime {
				// Reuse the old module
				mods = append(mods, *oldMod)
			} else {
				// Create a stub module without symbols
				curMod.setStub(moduleName)
				mods = append(mods, curMod)
				needReloadSymbols = true
			}
		}
	}

	sort.Slice(mods, func(i, j int) bool {
		return mods[i].start >= mods[j].start
	})
	// Heap allocate the exact amount needed. This also makes the initial
	// buffer stack allocated.
	s.modules.Store(slices.Clone(mods))

	return needReloadSymbols, nil
}

// reloadWorker is the goroutine handling the reloads of the kallsyms.
func (s *Symbolizer) reloadWorker(ctx context.Context, kobjectClient *kobject.Client) {
	noTimeout := make(<-chan time.Time)
	nextKallsymsReload := noTimeout
	nextModulesReload := noTimeout
	for {
		select {
		case <-s.reloadModules:
			// Just trigger reloading of modules with small delay to batch
			// potentially multiple module loads.
			if nextModulesReload == noTimeout {
				nextModulesReload = time.After(100 * time.Millisecond)
			}
		case <-nextModulesReload:
			if reloadSymbols, err := s.loadModules(); err == nil {
				log.Debugf("Kernel modules metadata reloaded, new symbols: %v", reloadSymbols)
				nextModulesReload = noTimeout
				if reloadSymbols && nextKallsymsReload == noTimeout {
					nextKallsymsReload = time.After(time.Minute)
				}
			} else {
				log.Warnf("Failed to reload kernel modules metadata: %v", err)
				nextModulesReload = time.After(10 * time.Second)
			}
		case <-nextKallsymsReload:
			if err := s.loadKallsyms(); err == nil {
				log.Debugf("Kernel symbols reloaded")
				nextKallsymsReload = noTimeout
			} else {
				log.Warnf("Failed to reload kernel symbols: %v", err)
				nextKallsymsReload = time.After(time.Minute)
			}
		case <-ctx.Done():
			// Terminate also the kobject poller thread
			_ = kobjectClient.Close()
			return
		}
	}
}

// pollKobjectClient listens for kernel kobject events to reload kallsyms when needed.
func (s *Symbolizer) pollKobjectClient(kobjectClient *kobject.Client) {
	for {
		event, err := kobjectClient.Receive()
		if err != nil {
			return
		}
		if event.Subsystem == "module" {
			log.Debugf("Kernel modules changed")
			// Notify worker thread without blocking
			select {
			case s.reloadModules <- true:
			default:
			}
		}
	}
}

// Reload will trigger asynchronous update of modules and symbols.
func (s *Symbolizer) StartMonitor(ctx context.Context) error {
	kobjectClient, err := kobject.New()
	if err != nil {
		return fmt.Errorf("failed to create kobject netlink socket: %v", err)
	}
	go s.reloadWorker(ctx, kobjectClient)
	go s.pollKobjectClient(kobjectClient)
	return nil
}

// getModuleByAddress is a helper to find a Module from the sorted 'modules'
// slice matching the address 'pc'.
func getModuleByAddress(modules []Module, pc libpf.Address) (*Module, error) {
	modIdx := sort.Search(len(modules), func(i int) bool {
		return pc >= modules[i].start
	})
	if modIdx >= len(modules) {
		return nil, ErrNoModule
	}
	m := &modules[modIdx]
	if pc < m.start || pc >= m.end {
		return nil, ErrNoModule
	}
	return m, nil
}

// GetModuleByAddress finds the Module containing the address 'pc'.
func (s *Symbolizer) GetModuleByAddress(pc libpf.Address) (*Module, error) {
	return getModuleByAddress(s.modules.Load().([]Module), pc)
}

// GetModuleByAddress finds the Module containing the module 'module'.
func (s *Symbolizer) GetModuleByName(module string) (*Module, error) {
	modules := s.modules.Load().([]Module)
	for i := range modules {
		kmod := &modules[i]
		if kmod.Name() == module {
			return kmod, nil
		}
	}
	return nil, ErrNoModule
}
