// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//! Library for zero-copy decoding of Go runtime information.
//!
//! The format is documented in detail [here](https://github.com/open-telemetry/opentelemetry-ebpf-profiler/blob/c96e126866/doc/gopclntab.md).

// If we ever want to support building for 32 bit, we'd need get rid of all
// the `as usize` casts and instead do checked conversions. Removing this
// check would not result in any safety hazards, but parsing of executables
// that don't fit memory of 32 bit machines would fail in quite unintuitive
// ways and debug builds might panic due to integer overflows.
#[cfg(not(target_pointer_width = "64"))]
compile_error!("gosym currently assumes sizeof(usize) == 8");

mod errors;
pub use errors::*;
mod raw;

use crate::{objfile, symbconv, VirtAddr};
use fallible_iterator::FallibleIterator;
use std::ops::Range;

/// Decoder for Go runtime information.
#[derive(Debug)]
pub struct GoRuntimeInfo<'obj> {
    header: raw::Header,
    text_start: VirtAddr,
    func_name_table: raw::FuncNameTable<'obj>,
    pc_table: raw::PcTable<'obj>,
    cu_table: raw::CuTable<'obj>,
    func_table: raw::FuncTable<'obj>,
    file_name_table: raw::FileNameTable<'obj>,
    func_data: raw::FuncData<'obj>,
}

impl<'obj> GoRuntimeInfo<'obj> {
    /// Locate Go runtime information in the given object file.
    pub fn open(obj: &objfile::Reader<'obj>) -> Result<Self> {
        if !obj.is_little_endian() {
            return Err(Error::BigEndian);
        }

        // Locate `.gopclntab` section.
        let mem = obj.memory_map()?;
        let (gopclntab_va, gopclntab) = Self::find_gopclntab(obj, &mem)?;

        // Reader the header.
        let header = raw::Header::read(gopclntab)?;
        let gopclntab = raw::Reader::new(header, gopclntab_va, gopclntab);
        let offsets = raw::HeaderOffsets::new(gopclntab.clone())?;

        // Go versions older than 1.16 are currently not supported.
        if header.version < Version::V116 {
            return Err(Error::UnsupportedGoVersion);
        }

        // Go >= 1.18 uses offsets relative to `go:func.*` for function data
        // whereas older Go versions simply emit absolute pointers.
        let func_data = if header.version >= Version::V118 {
            raw::FuncData::GoFunc(Self::find_gofunc(
                obj,
                &mem,
                gopclntab_va,
                header,
                &offsets,
            )?)
        } else {
            raw::FuncData::Global(header, mem)
        };

        // Fall back to code section address if the text start isn't
        // available from the header.
        let text_start = if let Some(start) = offsets.text_start {
            start
        } else {
            obj.load_section(b".text")?
                .ok_or(Error::CodeSectionNotFound)?
                .virt_addr()
        };

        // Create decoders for the various sub-regions of runtime info.
        Ok(Self {
            func_data,
            text_start,
            header,
            func_name_table: raw::FuncNameTable::new(&offsets, gopclntab.clone())?,
            file_name_table: raw::FileNameTable::new(&offsets, gopclntab.clone())?,
            func_table: raw::FuncTable::new(&offsets, gopclntab.clone())?,
            pc_table: raw::PcTable::new(&offsets, gopclntab.clone())?,
            cu_table: raw::CuTable::new(&offsets, gopclntab)?,
        })
    }

    /// Returns the Go runtime data version.
    pub fn version(&self) -> Version {
        self.header.version
    }

    /// Iterate over all top-level functions in the executable.    
    pub fn funcs<'rt>(&'rt self) -> Result<FuncIter<'rt, 'obj>> {
        Ok(FuncIter {
            rt: self,
            iter: self.func_table.index_iter()?,
        })
    }

    /// Locates the Go function containing the given virtual address.
    ///
    /// Returns:
    /// - `Ok(Some(Func))` if a function is found containing the address
    /// - `Ok(None)` if no function contains the address
    /// - `Err` if there was an error reading the function table
    pub fn find_func<'rt>(&'rt self, addr: VirtAddr) -> Result<Option<Func<'rt, 'obj>>> {
        Ok(self
            .func_table
            .func_by_addr(self.text_start, addr)?
            .map(|raw_func| Func {
                rt: self,
                raw: raw_func,
            }))
    }
}

/// Internal helpers.
impl<'obj> GoRuntimeInfo<'obj> {
    /// Locate the `go:func.*` memory region.
    fn find_gofunc(
        obj: &objfile::Reader<'obj>,
        mem: &objfile::MemoryMap<'obj>,
        gopclntab_va: VirtAddr,
        header: raw::Header,
        offsets: &raw::HeaderOffsets,
    ) -> Result<raw::Reader<'obj>> {
        // The region is pointed to only by the module data record, not by
        // the normal `.gopclntab` header. Need to locate module data first.
        let module_data = Self::find_module_data(obj, mem, gopclntab_va, header, &offsets)?;

        // NOTE: in newer Go versions there can be more than one module data
        //       record via the `next` field, but I'm not sure when this is
        //       used. Never seen it filled. Perhaps for Go shared libraries?

        let sec = mem
            .section_for_addr(module_data.go_func)
            .ok_or(Error::BadGoFuncPtr)?;
        let data = sec.as_obj_slice().ok_or(Error::CannotAvoidCopy)?;
        let offset = (module_data.go_func - sec.virt_addr()) as usize;
        let reader = raw::Reader::new(header, module_data.go_func, &data[offset..]);

        Ok(reader)
    }

    /// Locate the `.gopclntab` section.
    ///
    /// Uses section headers when present and falls back to an heuristic
    /// approach that scans for the known portions of the gopclntab header.
    fn find_gopclntab(
        obj: &objfile::Reader<'obj>,
        mem: &objfile::MemoryMap<'obj>,
    ) -> Result<(VirtAddr, &'obj [u8])> {
        // Try section headers first.
        for sec_name in [
            b".gopclntab".as_slice(),
            b".data.rel.ro.gopclntab".as_slice(),
        ] {
            if let Some(sec) = obj.load_section(sec_name)? {
                let data = sec.as_obj_slice().ok_or(Error::CannotAvoidCopy)?;
                return Ok((sec.virt_addr(), data));
            }
        }

        // Infer pointer size and quantum from architecture.
        let (ptr_size, quantum) = match obj.arch() {
            Some(objfile::Arch::X86_64) => (8, 1),
            Some(objfile::Arch::Aarch64) => (8, 4),
            None => return Err(Error::GopclntabNotFound),
        };

        // Scan all memory for header signature.
        for region in mem {
            // Scan with a stride of `ptr_size`: we expect the header
            // to be stored aligned to that at the very least.
            for (offs, window) in region.windows(8).enumerate().step_by(ptr_size.into()) {
                if &window[1..4] != b"\xFF\xFF\xFF" {
                    continue;
                }
                if &window[4..] != &[0, 0, quantum, ptr_size] {
                    continue;
                }
                if let Err(_) = Version::from_magic(window[..4].try_into().unwrap()) {
                    continue;
                }

                let va = region.virt_addr() + offs as u64;
                let slice = region.as_obj_slice().ok_or(Error::CannotAvoidCopy)?;
                let gopclntab = &slice[offs..];

                return Ok((va, gopclntab));
            }
        }

        Err(Error::GopclntabNotFound)
    }

    /// Locate and parse `runtime.firstmoduledata`.
    ///
    /// Uses object symbols when present and falls back to an heuristic
    /// approach that scans the executable for a known pattern if symbols
    /// aren't available.
    fn find_module_data(
        obj: &objfile::Reader<'_>,
        mem: &objfile::MemoryMap<'_>,
        gopclntab_va: VirtAddr,
        header: raw::Header,
        offsets: &raw::HeaderOffsets,
    ) -> Result<raw::ModuleData> {
        // Try via symbol lookup first. This is a `LOCAL` symbol that will
        // likely be stripped in most production executables, but it's worth
        // a try: the fallback path has to scan a lot of memory.
        if let Some(sym) = obj.resolve_symbol("runtime.firstmoduledata") {
            let sec = mem
                .section_for_addr(sym.virt_addr)
                .ok_or(Error::InvalidPtr)?;
            let slice = &sec[(sym.virt_addr - sec.virt_addr()) as usize..];
            let reader = raw::Reader::new(header, sym.virt_addr, slice);
            return raw::ModuleData::read(reader);
        }

        // No luck with symbols. Fall back to locating it via the pointer to
        // `.gopclntab` that it always starts with. Approach inspired by what
        // Stephen Eckels describes here:
        //
        // https://www.mandiant.com/resources/blog/golang-internals-symbol-recovery
        let needle = &gopclntab_va.to_le_bytes()[..header.ptr_size as usize];
        let expected_funcnametab = offsets.funcname_offset.0.wrapping_add(gopclntab_va);
        let expected_cutab = offsets.cutab_offset.0.wrapping_add(gopclntab_va);

        for region in mem {
            for (offs, window) in region
                .windows(needle.len())
                .enumerate()
                .step_by(header.ptr_size as usize)
            {
                if window != needle {
                    continue;
                }

                let addr = region.virt_addr().wrapping_add(offs as u64);
                let reader = raw::Reader::new(header, addr, &region[offs..]);
                let Ok(candidate) = raw::ModuleData::read(reader) else {
                    continue;
                };

                // Validate a few fields against gopclntab.
                if candidate.funcnametab != expected_funcnametab
                    || candidate.cutab != expected_cutab
                {
                    continue;
                }

                // Looking good!
                return Ok(candidate);
            }
        }

        Err(Error::ModuleDataNotFound)
    }
}

/// Version of the Go runtime information.
///
/// The data format usually stays the same for multiple Go versions.
/// We thus only list versions where significant changes occurred.
#[derive(Debug, Clone, Copy, PartialEq, Eq, PartialOrd, Ord)]
pub enum Version {
    /// Go v1.2 - v1.15.
    V12,
    /// Go 1.16 - v1.17.
    V116,
    /// Go 1.18 - 1.19.
    V118,
    /// Go 1.20 - latest as of writing.
    V120,
}

impl std::fmt::Display for Version {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.write_str(match self {
            Version::V12 => "v1.2",
            Version::V116 => "v1.16",
            Version::V118 => "v1.18",
            Version::V120 => "v1.20",
        })
    }
}

impl Version {
    fn from_magic(magic: [u8; 4]) -> Result<Version> {
        Ok(match &magic {
            b"\xFB\xFF\xFF\xFF" => Version::V12,
            b"\xFA\xFF\xFF\xFF" => Version::V116,
            b"\xF0\xFF\xFF\xFF" => Version::V118,
            b"\xF1\xFF\xFF\xFF" => Version::V120,
            _ => return Err(Error::UnsupportedGoVersion),
        })
    }
}

/// Iterator over all top-level functions in the executable.
pub struct FuncIter<'rt, 'obj> {
    rt: &'rt GoRuntimeInfo<'obj>,
    iter: raw::FuncIndexIter<'obj>,
}

impl<'rt, 'obj> FallibleIterator for FuncIter<'rt, 'obj> {
    type Item = Func<'rt, 'obj>;
    type Error = Error;

    fn next(&mut self) -> Result<Option<Self::Item>> {
        let Some(index_entry) = self.iter.next()? else {
            return Ok(None);
        };

        let raw = self.rt.func_table.func(index_entry.funcoff)?;
        Ok(Some(Func { rt: self.rt, raw }))
    }
}

/// Top-level function in the executable.
#[derive(Debug)]
pub struct Func<'rt, 'obj> {
    rt: &'rt GoRuntimeInfo<'obj>,
    raw: raw::Func<'obj>,
}

impl<'rt, 'obj> Func<'rt, 'obj> {
    /// Returns the start address of this function.
    pub fn start_addr(&self) -> VirtAddr {
        match self.raw.func_pc {
            raw::CodePtr::Addr(va) => va,
            raw::CodePtr::Offs(offs) => self.rt.text_start.wrapping_add(offs.0),
        }
    }

    /// Read the function name.
    pub fn name(&self) -> Result<&'obj str> {
        self.rt.func_name_table.name(self.raw.name)
    }

    /// Construct an iterator yielding mappings from PC to file names.
    pub fn file_mapping(&self) -> Result<PcFileIter<'rt, 'obj>> {
        Ok(PcFileIter {
            rt: self.rt,
            pc_base: self.start_addr(),
            cu_offset: self.raw.cu_offset,
            iter: self.rt.pc_table.pcdata(self.raw.pcfile)?,
        })
    }

    /// Construct an iterator yielding mappings from PC to line numbers.
    pub fn line_mapping(&self) -> Result<PcLineIter<'obj>> {
        Ok(PcLineIter {
            pc_base: self.start_addr(),
            iter: self.rt.pc_table.pcdata(self.raw.pcln)?,
        })
    }

    /// First line of the function definition.
    ///
    /// Only available for Go >= v1.20.
    pub fn start_line(&self) -> Option<u32> {
        self.raw.start_line
    }

    /// Construct an iterator yielding mappings from PC to the deepest inline
    /// function in the inline tree.
    ///
    /// You can then use [`InlinedCall::parent_pc`] to figure out the parents
    /// inline function (doing another pass through the inline mapping).
    pub fn inline_mapping(&self) -> Result<Option<InlineTreeIter<'rt, 'obj>>> {
        use raw::{FuncDataField::*, PcDataField::*};

        let Some(inline_tree) = self.raw.func_data(InlTree) else {
            return Ok(None);
        };
        let Some(index_pcdata) = self.raw.pc_data(InlTreeIndex) else {
            return Ok(None);
        };

        Ok(Some(InlineTreeIter {
            rt: self.rt,
            inline_tree,
            pc_base: self.start_addr(),
            iter: self.rt.pc_table.pcdata(index_pcdata)?,
        }))
    }
}

/// Iterator over mappings from PCs to the file name
/// that the code was generated from.
#[derive(Debug)]
pub struct PcFileIter<'rt, 'obj> {
    rt: &'rt GoRuntimeInfo<'obj>,
    pc_base: VirtAddr,
    cu_offset: raw::CuTabIndex,
    iter: raw::PcDataReader<'obj>,
}

impl<'rt, 'obj> FallibleIterator for PcFileIter<'rt, 'obj> {
    type Item = (Range<VirtAddr>, Option<&'obj str>);
    type Error = Error;

    fn next(&mut self) -> Result<Option<Self::Item>> {
        let Some((pc_offs, file_ref)) = self.iter.next()? else {
            return Ok(None);
        };

        let pc = range_rel2abs(self.pc_base, pc_offs);

        let cu_file_idx = match file_ref {
            i32::MIN..=-2 => return Err(Error::BadFileIndex),
            -1 => return Ok(Some((pc, None))),
            0..=i32::MAX => raw::CuTabIndex(file_ref as u32),
        };

        let offs = self
            .rt
            .cu_table
            .file_name_offset(self.cu_offset, cu_file_idx)?;

        let file = if offs != raw::FileNameOffset::INVALID {
            Some(self.rt.file_name_table.name(offs)?)
        } else {
            None
        };

        Ok(Some((pc, file)))
    }
}

/// Iterator over mappings from PCs to the line number
/// that the code was generated from.
#[derive(Debug)]
pub struct PcLineIter<'obj> {
    pc_base: VirtAddr,
    iter: raw::PcDataReader<'obj>,
}

impl<'obj> FallibleIterator for PcLineIter<'obj> {
    type Item = (Range<VirtAddr>, Option<u32>);
    type Error = Error;

    fn next(&mut self) -> Result<Option<Self::Item>> {
        let Some((pc_offs, raw_line)) = self.iter.next()? else {
            return Ok(None);
        };

        let pc = range_rel2abs(self.pc_base, pc_offs);

        let line = match raw_line {
            i32::MIN..=-2 | 0 => return Err(Error::BadLineNumber),
            -1 => None,
            0..=i32::MAX => Some(raw_line as u32),
        };

        Ok(Some((pc, line)))
    }
}

/// Iterator over mappings from PCs to inline calls.
#[derive(Debug)]
pub struct InlineTreeIter<'rt, 'obj> {
    rt: &'rt GoRuntimeInfo<'obj>,
    inline_tree: raw::FuncDataRef,
    pc_base: VirtAddr,
    iter: raw::PcDataReader<'obj>,
}

impl<'rt, 'obj> FallibleIterator for InlineTreeIter<'rt, 'obj> {
    type Item = (Range<VirtAddr>, Option<InlinedCall<'rt, 'obj>>);
    type Error = Error;

    fn next(&mut self) -> Result<Option<Self::Item>> {
        let Some((pc_offs, tree_idx)) = self.iter.next()? else {
            return Ok(None);
        };

        let pc = range_rel2abs(self.pc_base, pc_offs);

        let tree_idx = match tree_idx {
            i32::MIN..=-2 => return Err(Error::BadInlineIndex),
            -1 => return Ok(Some((pc, None))),
            0..=i32::MAX => raw::InlineTreeIndex(tree_idx as u32),
        };

        let call = InlinedCall {
            rt: self.rt,
            outer_fn_entry: self.pc_base,
            raw: self.rt.func_data.inlined_call(self.inline_tree, tree_idx)?,
        };

        Ok(Some((pc, Some(call))))
    }
}

/// Represents a function that got inlined into a top-level function.
#[derive(Debug)]
pub struct InlinedCall<'rt, 'obj> {
    rt: &'rt GoRuntimeInfo<'obj>,
    outer_fn_entry: VirtAddr,
    raw: raw::InlinedCall,
}

impl<'rt, 'obj> InlinedCall<'rt, 'obj> {
    /// Read the name of the function that got inlined.
    pub fn name(&self) -> Result<&'obj str> {
        self.rt.func_name_table.name(self.raw.name_offset)
    }

    /// Gets the first line number of the inlined function.
    ///
    /// Only available for Go >= 1.20.
    pub fn start_line(&self) -> Option<u32> {
        match &self.raw.info {
            raw::InlinedCallInfo::New { start_line, .. } => Some(*start_line),
            raw::InlinedCallInfo::Old { .. } => None,
        }
    }

    /// Gets the address of the next higher function in the inline chain.
    pub fn parent_pc(&self) -> VirtAddr {
        self.outer_fn_entry.wrapping_add(self.raw.parent_pc)
    }
}

/// Makes a relative PC offset range absolute.
fn range_rel2abs(base: VirtAddr, rng: Range<raw::TextStartOffset>) -> Range<VirtAddr> {
    Range {
        start: base.wrapping_add(rng.start.0),
        end: base.wrapping_add(rng.end.0),
    }
}

impl symbconv::PointResolver for GoRuntimeInfo<'_> {
    /// NOTE: this is currently doesn't support inline functions
    fn symbols_for_pc(&self, pc: VirtAddr) -> symbconv::Result<Vec<symbconv::ResolvedSymbol>> {
        let func = match self.find_func(pc) {
            Ok(Some(func)) => func,
            Ok(None) => return Ok(Vec::new()),
            Err(e) => return Err(symbconv::Error::Go(symbconv::go::Error::Gosym(e))),
        };

        let mut symbols = Vec::new();
        let mut source_file = None;
        let mut line_number = None;

        // For file mappings
        let mut file_iter = func
            .file_mapping()
            .map_err(|e| symbconv::Error::Go(symbconv::go::Error::Gosym(e)))?;
        while let Ok(Some((range, file))) = file_iter.next() {
            if range.contains(&VirtAddr::from(pc)) {
                source_file = Some(file.unwrap_or("<unknown>").into());
                break;
            }
        }

        // For line mappings
        let mut line_iter = func
            .line_mapping()
            .map_err(|e| symbconv::Error::Go(symbconv::go::Error::Gosym(e)))?;
        while let Ok(Some((range, line))) = line_iter.next() {
            if range.contains(&VirtAddr::from(pc)) {
                line_number = Some(line.unwrap_or(0));
                break;
            }
        }

        symbols.push(symbconv::ResolvedSymbol {
            start_addr: func.start_addr(),
            function_name: func.name().ok().map(|s| s.to_string()),
            file_name: source_file,
            line_number: line_number,
        });

        Ok(symbols)
    }
}
