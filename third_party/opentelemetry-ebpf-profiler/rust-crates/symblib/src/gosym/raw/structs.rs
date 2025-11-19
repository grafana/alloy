// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//! Decoding for Go's runtime data structures.
//
//! Types in this module are all concerned with each decoding a single data
//! structure. If one data structure contains references into another, this
//! is represented as an offset. Chasing offsets and following references
//! is left to the main module.

use super::*;
use fallible_iterator::FallibleIterator;
use std::ops::Range;

/// Minimal subset of the gopclntab header (`runtime.pcData`).
///
/// This contains all the fields that [`Reader`] needs to know how to read Go
/// specific types but not any more. The intention is to keep this type small
/// enough that we can pass it around in a register.
#[derive(Debug, Clone, Copy)]
pub struct Header {
    pub version: Version,
    pub quantum: u8,
    pub ptr_size: u8,
}

impl Header {
    pub fn read(sec: &[u8]) -> Result<Self> {
        // Check header: 4-byte magic, two zeros, pc quantum, pointer size.
        if sec.len() < 16 || sec[4] != 0 || sec[5] != 0 {
            return Err(Error::MalformedGopclntab);
        }

        let version = Version::from_magic(sec[..4].try_into().unwrap())?;

        // quantum and ptrSize are the same between 1.2, 1.16, and 1.18
        let quantum = sec[6];
        if !matches!(quantum, 1 | 2 | 4) {
            return Err(Error::MalformedGopclntab);
        }

        let ptr_size = sec[7];
        if !matches!(ptr_size, 4 | 8) {
            return Err(Error::MalformedGopclntab);
        }

        Ok(Header {
            version,
            quantum,
            ptr_size,
        })
    }
}

/// Rest of the `.gopclntab` header (`runtime.pcData`).
///
/// Excluding the portion that we already have via [`Header`].
#[derive(Debug)]
pub struct HeaderOffsets {
    pub num_funcs: u64,
    #[allow(dead_code)]
    pub num_files: u64,
    pub text_start: Option<VirtAddr>,
    pub funcname_offset: GopclntabOffset,
    pub cutab_offset: GopclntabOffset,
    pub filetab_offset: GopclntabOffset,
    pub pctab_offset: GopclntabOffset,
    pub pcln_offset: GopclntabOffset,
}

impl HeaderOffsets {
    pub fn new(mut r: Reader<'_>) -> Result<Self> {
        Ok(Self {
            num_funcs: r.skip(8).uintptr()?,
            num_files: r.uintptr()?,
            text_start: if r.version() >= Version::V118 {
                Some(r.uintptr()?)
            } else {
                None
            },
            funcname_offset: GopclntabOffset(r.uintptr()?),
            cutab_offset: GopclntabOffset(r.uintptr()?),
            filetab_offset: GopclntabOffset(r.uintptr()?),
            pctab_offset: GopclntabOffset(r.uintptr()?),
            pcln_offset: GopclntabOffset(r.uintptr()?),
        })
    }
}

/// Decoder for the `runtime.functab` structure.
///
/// <https://github.com/golang/go/blob/go1.16.15/src/runtime/symtab.go#L500>
/// <https://github.com/golang/go/blob/go1.20.6/src/runtime/symtab.go#L582>
#[derive(Debug)]
pub struct FuncTabIndexEntry {
    #[allow(dead_code)]
    pub entry: CodePtr,
    pub funcoff: FuncTabOffset,
}

impl FuncTabIndexEntry {
    pub fn size_of(h: Header) -> usize {
        if h.version >= Version::V118 {
            2 * 4
        } else {
            h.ptr_size as usize * 2
        }
    }

    pub fn read(r: &mut Reader<'_>) -> Result<Self> {
        Ok(Self {
            entry: r.code_ptr()?,
            funcoff: FuncTabOffset(if r.version() >= Version::V118 {
                r.u32()? as u64
            } else {
                r.uintptr()?
            }),
        })
    }
}

/// Index in the dynamic PC data array in [`Func`].
///
/// <https://github.com/golang/go/blob/go1.21rc3/src/internal/abi/symtab.go#L76>
#[allow(dead_code)]
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum PcDataField {
    InlTreeIndex = 2,
}

/// Index in the dynamic func data array in [`Func`].
///
/// <https://github.com/golang/go/blob/go1.21rc3/src/internal/abi/symtab.go#L76>
#[allow(dead_code)]
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum FuncDataField {
    InlTree = 3,
}

/// Decoder for the `runtime._func` structure.
///
/// <https://github.com/golang/go/blob/go1.16.15/src/runtime/runtime2.go#L822>
/// <https://github.com/golang/go/blob/go1.18.10/src/runtime/runtime2.go#L859>
/// <https://github.com/golang/go/blob/go1.20.6/src/runtime/runtime2.go#L882>
#[derive(Debug)]
pub struct Func<'obj> {
    pub func_pc: CodePtr,
    pub name: FuncNameOffset,
    pub pcfile: PcTabOffset,
    pub pcln: PcTabOffset,
    pub cu_offset: CuTabIndex,
    pub start_line: Option<u32>,
    pc_data: Reader<'obj>,
    func_data: Reader<'obj>,
}

impl<'obj> Func<'obj> {
    pub fn read(mut r: Reader<'obj>) -> Result<Self> {
        use Version as V;

        let func_pc = r.code_ptr()?;
        let name = FuncNameOffset(r.u32()?);
        let pcfile = PcTabOffset(r.skip(4 * 3).u32()?);
        let pcln = PcTabOffset(r.u32()?);
        let npcdata = r.u32()? as usize;
        let cu_offset = CuTabIndex(r.u32()?);

        let start_line = if r.version() >= V::V120 {
            Some(r.u32()?)
        } else {
            None
        };

        r.skip(3); // flags + func ID + pad byte

        let nfuncdata = r.u8()? as usize;

        let pc_data = r.sub_reader(..4 * npcdata)?;
        r.skip(4 * npcdata);

        let func_data_offs_sz = if r.version() >= V::V118 {
            4
        } else {
            // For older Go versions we need to account for pointer alignment.
            r.align_up();
            r.ptr_size()
        };

        Ok(Func {
            func_pc,
            name,
            pcfile,
            pcln,
            cu_offset,
            pc_data,
            start_line,
            func_data: r.sub_reader(..func_data_offs_sz * nfuncdata)?,
        })
    }

    pub fn pc_data(&self, n: PcDataField) -> Option<PcTabOffset> {
        let mut r = self.pc_data.sub_reader(4 * n as usize..).ok()?;
        r.u32().ok().filter(|x| *x != 0).map(PcTabOffset)
    }

    pub fn func_data(&self, n: FuncDataField) -> Option<FuncDataRef> {
        if self.func_data.version() >= Version::V118 {
            let mut r = self.func_data.sub_reader(4 * n as usize..).ok()?;
            let offs = GoFuncOffset(r.u32().ok()?);
            if offs != GoFuncOffset::INVALID {
                Some(FuncDataRef::Offs(offs))
            } else {
                None
            }
        } else {
            let psz = self.func_data.ptr_size();
            let mut r = self.func_data.sub_reader(psz * n as usize..).ok()?;
            let addr = r.uintptr().ok()?;
            if addr != 0 {
                Some(FuncDataRef::Addr(addr))
            } else {
                None
            }
        }
    }
}

/// Decoder for `runtime.inlinedCall`
///
/// <https://github.com/golang/go/blob/go1.18.10/src/runtime/symtab.go#L1172>
/// <https://github.com/golang/go/blob/go1.20.6/src/runtime/symtab.go#L1208>
#[derive(Debug)]
pub struct InlinedCall {
    /// Marker for special runtime functions.
    #[allow(dead_code)]
    pub func_id: FuncId,

    /// Position of an instruction whose source position is the call site (offset from entry)
    pub parent_pc: u64,

    /// Offset into `runtime.funcname` for named of called function.
    ///
    /// The comment in the Go source says it's relative to `pclntab`,
    /// but that's clearly incorrect.
    pub name_offset: FuncNameOffset,

    /// Version specific data.
    pub info: InlinedCallInfo,
}

impl InlinedCall {
    pub fn size_of(h: Header) -> usize {
        if h.version >= Version::V120 {
            16
        } else {
            20
        }
    }

    fn read_new(mut r: Reader<'_>) -> Result<Self> {
        let func_id = FuncId(r.u8()?);
        let name_offset = FuncNameOffset(r.skip(3 /* pad */).u32()?);
        let parent_pc = r.u32()? as u64;
        let start_line = r.u32()?;

        Ok(InlinedCall {
            func_id,
            parent_pc,
            name_offset,
            info: InlinedCallInfo::New { start_line },
        })
    }

    fn read_old(mut r: Reader<'_>) -> Result<Self> {
        let parent_idx = r.i16()?;
        let func_id = FuncId(r.u8()?);
        let file = CuTabIndex(r.skip(1 /* pad */).u32()?); // TODO: i32?
        let line = r.u32()?;
        let name_offset = FuncNameOffset(r.u32()?);
        let parent_pc = r.u32()? as u64;

        Ok(InlinedCall {
            func_id,
            parent_pc,
            name_offset,
            info: InlinedCallInfo::Old {
                parent_idx,
                file,
                line,
            },
        })
    }

    pub fn read(r: Reader<'_>) -> Result<Self> {
        if r.version() >= Version::V120 {
            Self::read_new(r)
        } else {
            Self::read_old(r)
        }
    }
}

/// Version specific portion of [`InlinedCall`].
#[derive(Debug)]
pub enum InlinedCallInfo {
    New {
        /// Line number of start of function (func keyword/TEXT directive).
        start_line: u32,
    },
    Old {
        /// Index of parent in the inline tree, or < 0.
        #[allow(dead_code)]
        parent_idx: i16,
        /// Per-CU file index for inlined call.
        #[allow(dead_code)]
        file: CuTabIndex,
        /// Line number of the call site.
        #[allow(dead_code)]
        line: u32,
    },
}

/// Decoder for data from `runtime.moduledata`.
///
/// <https://github.com/golang/go/blob/go1.18.10/src/runtime/symtab.go#L415>
/// <https://github.com/golang/go/blob/go1.20.6/src/runtime/symtab.go#L434>
#[derive(Debug)]
pub struct ModuleData {
    /// Address of the function name table.
    pub funcnametab: VirtAddr,
    /// Address of the CU table.
    pub cutab: VirtAddr,
    /// Start of the `go:func.*` region.
    pub go_func: VirtAddr,
}

impl ModuleData {
    /// Read module data from the given reader.
    pub fn read(r: Reader<'_>) -> Result<Self> {
        // offsetof(..) for funcnametab.ptr, cutab.ptr and gofunc fields
        let (funcnametab, cutab, go_func);

        match r.version() {
            Version::V118 => {
                funcnametab = 1 * r.ptr_size();
                cutab = 4 * r.ptr_size();
                go_func = 38 * r.ptr_size();
            }
            Version::V120 => {
                funcnametab = 1 * r.ptr_size();
                cutab = 4 * r.ptr_size();
                go_func = 40 * r.ptr_size();
            }
            _ => return Err(Error::UnsupportedGoVersion),
        }

        Ok(ModuleData {
            funcnametab: r.sub_reader(funcnametab..)?.uintptr()?,
            cutab: r.sub_reader(cutab..)?.uintptr()?,
            go_func: r.sub_reader(go_func..)?.uintptr()?,
        })
    }
}

/// Decoder for `pcdata` sequences within [`PcTable`].
///
/// <https://github.com/golang/go/blob/go1.16.15/src/cmd/internal/objfile/goobj.go#L284>
#[derive(Debug)]
pub struct PcDataReader<'obj> {
    reader: Option<Reader<'obj>>,
    pc_offset: TextStartOffset,
    value: i32,
    first: bool,
}

impl<'obj> PcDataReader<'obj> {
    pub fn new(r: Reader<'obj>) -> Self {
        PcDataReader {
            pc_offset: TextStartOffset(0),
            value: -1,
            first: true,
            reader: Some(r),
        }
    }
}

impl<'obj> FallibleIterator for PcDataReader<'obj> {
    type Item = (Range<TextStartOffset>, i32);
    type Error = Error;

    fn next(&mut self) -> Result<Option<Self::Item>> {
        let Some(reader) = self.reader.as_mut() else {
            return Ok(None);
        };

        let uv_delta = reader.var_i32()?;
        if uv_delta == 0 && !self.first {
            self.reader = None;
            return Ok(None);
        }
        self.value = self.value.wrapping_add(uv_delta);

        let pc_delta = reader.var_u32()? as u64;
        let pc_delta_scaled = pc_delta.wrapping_mul(reader.quantum() as u64);
        let prev_pc_offs = self.pc_offset;
        self.pc_offset.0 = prev_pc_offs.0.wrapping_add(pc_delta_scaled);

        self.first = false;
        Ok(Some((prev_pc_offs..self.pc_offset, self.value)))
    }
}
