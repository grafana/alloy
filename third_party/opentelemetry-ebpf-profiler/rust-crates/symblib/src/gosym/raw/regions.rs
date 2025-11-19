// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//! Reference types for sub-regions of the Go runtime information.
//!
//! Most of these represent sub-regions of the `.gopclntab` section, but
//! there are some exceptions where data lives in other sections.

use super::*;
use crate::objfile;
use fallible_iterator::FallibleIterator;

/// Decoder for `.gopclntab`:`runtime.filetab`.
#[derive(Debug)]
pub struct FileNameTable<'obj>(Reader<'obj>);

impl<'obj> FileNameTable<'obj> {
    pub fn new(o: &HeaderOffsets, gopclntab: Reader<'obj>) -> Result<Self> {
        Ok(Self(gopclntab.sub_reader(o.filetab_offset.0 as usize..)?))
    }

    pub fn name(&self, offset: FileNameOffset) -> Result<&'obj str> {
        self.0.sub_reader(offset.0 as usize..)?.str()
    }
}

/// Decoder for `.gopclntab`:`runtime.cutab`.
#[derive(Debug)]
pub struct CuTable<'obj>(Reader<'obj>);

impl<'obj> CuTable<'obj> {
    pub fn new(o: &HeaderOffsets, gopclntab: Reader<'obj>) -> Result<Self> {
        gopclntab.sub_reader(o.cutab_offset.0 as usize..).map(Self)
    }

    pub fn file_name_offset(
        &self,
        cu_idx: CuTabIndex,
        fn_idx: CuTabIndex,
    ) -> Result<FileNameOffset> {
        let offs = (cu_idx.0 as u64 + fn_idx.0 as u64) * 4;
        self.0
            .sub_reader(offs as usize..)?
            .u32()
            .map(FileNameOffset)
    }
}

/// Decoder for `.gopclntab`:`runtime.funcnametab`.
#[derive(Debug)]
pub struct FuncNameTable<'obj>(Reader<'obj>);

impl<'obj> FuncNameTable<'obj> {
    pub fn new(o: &HeaderOffsets, gopclntab: Reader<'obj>) -> Result<Self> {
        gopclntab
            .sub_reader(o.funcname_offset.0 as usize..)
            .map(Self)
    }

    pub fn name(&self, offset: FuncNameOffset) -> Result<&'obj str> {
        self.0.sub_reader(offset.0 as usize..)?.str()
    }
}

/// Decoder for `.gopclntab`:`runtime.pctab`.
///
/// Note that while Go calls this a "table" it is actually just a
/// concatenation of `pcdata` sequences (see [`PcDataReader`]).
#[derive(Debug)]
pub struct PcTable<'obj>(Reader<'obj>);

impl<'obj> PcTable<'obj> {
    pub fn new(o: &HeaderOffsets, gopclntab: Reader<'obj>) -> Result<Self> {
        gopclntab.sub_reader(o.pctab_offset.0 as usize..).map(Self)
    }

    pub fn pcdata(&self, offset: PcTabOffset) -> Result<PcDataReader<'obj>> {
        self.0
            .sub_reader(offset.0 as usize..)
            .map(PcDataReader::new)
    }
}

/// Decoder for `.gopclntab`:`runtime.functab`.
///
/// `runtime.functab`, in this case, refers to the label with that name that
/// the linker emits, not the structure type with the same name.
#[derive(Debug)]
pub struct FuncTable<'obj> {
    reader: Reader<'obj>,
    num_funcs: u64,
}

impl<'obj> FuncTable<'obj> {
    pub fn new(o: &HeaderOffsets, gopclntab: Reader<'obj>) -> Result<Self> {
        Ok(Self {
            reader: gopclntab.sub_reader(o.pcln_offset.0 as usize..)?,
            num_funcs: o.num_funcs,
        })
    }

    pub fn index_iter(&self) -> Result<FuncIndexIter<'obj>> {
        let sz = FuncTabIndexEntry::size_of(self.reader.header());
        let reader = self.reader.sub_reader(..sz * self.num_funcs as usize)?;
        Ok(FuncIndexIter(reader))
    }

    pub fn func(&self, offs: FuncTabOffset) -> Result<Func<'obj>> {
        Func::read(self.reader.sub_reader(offs.0 as usize..)?)
    }

    // Look up function information for a virtual address using binary search.
    pub fn func_by_addr(&self, text_start: VirtAddr, addr: VirtAddr) -> Result<Option<Func<'obj>>> {
        let sz = FuncTabIndexEntry::size_of(self.reader.header());
        let mut left = 0;
        let mut right = self.num_funcs as usize * sz;

        while left < right {
            let mid = (left + (right - left) / 2) / sz * sz;
            let mut reader = self.reader.sub_reader(mid..)?;
            let entry = FuncTabIndexEntry::read(&mut reader)?;

            let entry_addr = match entry.entry {
                CodePtr::Addr(addr) => addr,
                CodePtr::Offs(offset) => text_start + offset.0,
            };

            if addr < entry_addr {
                right = mid;
            } else if mid + sz < right {
                let mut next_reader = self.reader.sub_reader(mid + sz..)?;
                let next_entry = FuncTabIndexEntry::read(&mut next_reader)?;
                let next_addr = match next_entry.entry {
                    CodePtr::Addr(addr) => addr,
                    CodePtr::Offs(offset) => text_start + offset.0,
                };

                if addr >= next_addr {
                    left = mid + sz;
                } else {
                    return Ok(Some(self.func(entry.funcoff)?));
                }
            } else {
                return Ok(Some(self.func(entry.funcoff)?));
            }
        }

        Ok(None)
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::gosym::GoRuntimeInfo;
    use crate::tests::go_testdata;

    #[test]
    fn test_func_by_addr() -> Result<()> {
        for test_file in go_testdata() {
            let obj = objfile::File::load(&test_file)?;
            let obj = obj.parse()?;

            let runtime_info = GoRuntimeInfo::open(&obj)?;

            let text_start = obj.load_section(b".text")?.unwrap().virt_addr();

            for pc in (text_start..text_start + 0x10000).step_by(19) {
                let bin_search_result = runtime_info.find_func(pc).unwrap();

                let mut fn_iter = runtime_info.funcs().unwrap().peekable();
                let mut found = None;
                while let Some(func) = fn_iter.next().unwrap() {
                    let Some(next) = fn_iter.peek().unwrap() else {
                        break;
                    };

                    let pc_rng = func.start_addr()..next.start_addr();
                    if pc_rng.contains(&pc) {
                        found = Some(func);
                        break;
                    }
                }

                assert_eq!(
                    bin_search_result.map(|x| x.start_addr()),
                    found.map(|x| x.start_addr())
                );
            }
        }
        Ok(())
    }
}

/// Iterator over the index in `.gopclntab`:`runtime.functab`.
#[derive(Debug)]
pub struct FuncIndexIter<'obj>(Reader<'obj>);

impl FallibleIterator for FuncIndexIter<'_> {
    type Item = FuncTabIndexEntry;
    type Error = Error;

    fn next(&mut self) -> Result<Option<Self::Item>> {
        if self.0.is_empty() {
            return Ok(None);
        }

        FuncTabIndexEntry::read(&mut self.0).map(Some)
    }
}

/// Decoder for the `go:func.*` region.
#[derive(Debug)]
pub enum FuncData<'obj> {
    /// `go:func.*` references are absolute pointers (Go < 1.18).
    Global(Header, objfile::MemoryMap<'obj>),

    /// `go:func.*` references are relative to `gofunc` field in module data (Go >= 1.18).
    GoFunc(Reader<'obj>),
}

impl<'obj> FuncData<'obj> {
    fn mk_reader(&self, fdref: FuncDataRef) -> Result<Reader<'obj>> {
        match (fdref, self) {
            (FuncDataRef::Addr(abs), FuncData::Global(header, mem)) => {
                let sec = mem.section_for_addr(abs).ok_or(Error::InvalidPtr)?;
                let slice = sec.as_obj_slice().ok_or(Error::CannotAvoidCopy)?;
                let sub = &slice[abs as usize - sec.virt_addr() as usize..];
                Ok(Reader::new(*header, abs, sub))
            }
            (FuncDataRef::Offs(offs), FuncData::GoFunc(gofunc)) => {
                Ok(gofunc.sub_reader(offs.0 as usize..)?)
            }
            _ => unreachable!("bug: invalid addr/offs global/gofunc combination"),
        }
    }

    pub fn inlined_call(&self, tree: FuncDataRef, idx: InlineTreeIndex) -> Result<InlinedCall> {
        let mut reader = self.mk_reader(tree)?;
        let sz = InlinedCall::size_of(reader.header());
        reader.skip(idx.0 as usize * sz);
        InlinedCall::read(reader)
    }
}
