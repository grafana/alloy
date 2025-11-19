// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//! Helpers for reading Go's data structures.

use super::*;
use std::collections::Bound;
use std::ffi::CStr;
use std::ops::RangeBounds;
use std::slice::SliceIndex;

/// Reader ("cursor") type for reading Go data structures.
#[derive(Clone)]
pub struct Reader<'obj> {
    header: Header,
    addr: VirtAddr,
    data: &'obj [u8],
}

/// Implements a read function for each given primitive integer type.
macro_rules! impl_read_prim {
    ( $($ty:ident),* ) => {$(
        #[doc=concat!("Reads the next `", stringify!($ty), "`.")]
        pub fn $ty(&mut self) -> Result<$ty> {
            let sz = std::mem::size_of::<$ty>();

            if self.data.len() < sz {
                return Err(Error::UnexpectedEof);
            }

            let r = self.data[..sz].try_into().unwrap();
            let v = $ty::from_le_bytes(r);

            self.skip(sz);

            Ok(v)
        }
    )*}
}

impl<'obj> Reader<'obj> {
    /// Create a new reader from a slice and a header.
    pub fn new(header: Header, addr: VirtAddr, data: &'obj [u8]) -> Self {
        Self { header, addr, data }
    }

    /// Creates a new reader for a region within this reader.
    pub fn sub_reader<T>(&self, rng: T) -> Result<Self>
    where
        T: Clone + RangeBounds<usize> + SliceIndex<[u8], Output = [u8]>,
    {
        let mut new = self.clone();

        new.data = new.data.get(rng.clone()).ok_or(Error::UnexpectedEof)?;

        let start_offset = match rng.start_bound() {
            Bound::Included(x) => *x,
            Bound::Excluded(_) => unreachable!("start bound cannot be excluded"),
            Bound::Unbounded => 0,
        };

        new.addr = new.addr.wrapping_add(start_offset as u64);

        Ok(new)
    }

    /// Skip `n` bytes.
    pub fn skip(&mut self, n: usize) -> &mut Self {
        self.data = &self.data[n.min(self.data.len())..];
        self.addr = self.addr.wrapping_add(n as u64);
        self
    }

    /// Align to next multiple of pointer size.
    pub fn align_up(&mut self) -> &mut Self {
        let dangling = self.addr as usize % self.ptr_size();
        if dangling != 0 {
            self.skip(self.ptr_size() - dangling);
        }
        self
    }

    impl_read_prim!(u8, u32, u64, i16);

    /// Read the next pointer-sized integer.
    pub fn uintptr(&mut self) -> Result<u64> {
        Ok(match self.header.ptr_size {
            4 => self.u32()? as u64,
            8 => self.u64()?,
            _ => unreachable!("pre-checked on construction"),
        })
    }

    /// Reads the next code pointer.
    pub fn code_ptr(&mut self) -> Result<CodePtr> {
        Ok(if self.version() >= Version::V118 {
            CodePtr::Offs(TextStartOffset(self.u32()? as u64))
        } else {
            CodePtr::Addr(self.uintptr()?.wrapping_mul(self.quantum() as u64))
        })
    }

    /// Read a zero-terminated string.
    pub fn str(&mut self) -> Result<&'obj str> {
        let str = CStr::from_bytes_until_nul(self.data)
            .map_err(|_| Error::UnexpectedEof)?
            .to_str()
            .map_err(|_| Error::NonUtf8String)?;
        self.skip(str.len() + 1);
        Ok(str)
    }

    /// Reads a variable-length encoded `u32`.
    pub fn var_u32(&mut self) -> Result<u32> {
        let mut v = 0;
        for shift in (0..=31).step_by(7) {
            let b = self.u8()? as u32;
            v |= (b & 0x7F) << shift;
            if b & 0x80 == 0 {
                if shift == 4 * 7 && b & 0b0111_0000 != 0 {
                    return Err(Error::VarIntTooLong);
                }
                return Ok(v);
            }
        }
        Err(Error::VarIntTooLong)
    }

    /// Reads a zig-zag variable-length encoded `i32`.
    pub fn var_i32(&mut self) -> Result<i32> {
        let zigzag = self.var_u32()? as i32;
        Ok(-(zigzag & 1) ^ (zigzag >> 1))
    }

    /// Returns true if the reader doesn't have any data left.
    pub fn is_empty(&self) -> bool {
        self.data.is_empty()
    }

    /// Gets the whole header.
    pub fn header(&self) -> Header {
        self.header
    }

    /// Gets the Go version.
    pub fn version(&self) -> Version {
        self.header.version
    }

    /// Gets the pointer size.
    pub fn ptr_size(&self) -> usize {
        self.header.ptr_size as usize
    }

    /// Gets the code pointer quantum.
    pub fn quantum(&self) -> usize {
        self.header.quantum as usize
    }
}

/// Custom debug impl to prevent printing huge byte arrays.
impl std::fmt::Debug for Reader<'_> {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        write!(f, "Reader({} bytes @ {:#08X})", self.data.len(), self.addr)
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    static HEADER: Header = Header {
        version: Version::V120,
        quantum: 1,
        ptr_size: 8,
    };

    #[test]
    fn read_primitives() {
        let data: Vec<_> = (0..=0xff).collect();
        let mut reader = Reader::new(HEADER, 0, &data);
        assert_eq!(reader.u32().unwrap(), 0x03020100);
        assert_eq!(reader.u8().unwrap(), 0x04);
        assert_eq!(reader.skip(3).u64().unwrap(), 0xf0e0d0c0b0a0908);
    }

    #[test]
    fn sub_reader() {
        let data = b"\xAA\xBB\xCC\xDD\xEE\xFF";
        let all = Reader::new(HEADER, 0, data);

        {
            let mut sub = all.sub_reader(..2).unwrap();
            assert_eq!(sub.skip(1).u8().unwrap(), 0xBB);
            assert!(sub.is_empty());
        }

        {
            let mut sub = all.sub_reader(3..4).unwrap();
            assert_eq!(sub.u8().unwrap(), 0xDD);
            assert!(sub.is_empty());
        }

        {
            let mut sub = all.sub_reader(4..).unwrap();
            assert_eq!(sub.u8().unwrap(), 0xEE);
            assert_eq!(sub.u8().unwrap(), 0xFF);
            assert!(sub.is_empty());
        }

        for offs in (data.len() - 3)..(data.len() + 20) {
            assert!(all.sub_reader(offs..offs + 4).is_err(), "{offs}");
        }
    }

    #[test]
    fn var_u32() {
        let r = |x| Reader::new(HEADER, 0, x).var_u32();

        assert!(matches!(r(b"\xe5\x8e\xa6"), Err(Error::UnexpectedEof)));
        assert!(matches!(r(b""), Err(Error::UnexpectedEof)));
        assert!(matches!(r(b"\x95\x9a\xef\x3a"), Ok(123456789)));
        assert!(matches!(r(b"\xff\xff\xff\xff\x0f"), Ok(u32::MAX)));
        assert!(matches!(
            r(b"\xff\xff\xff\xff\x10"),
            Err(Error::VarIntTooLong)
        ));

        assert!(matches!(r(b"\x00"), Ok(0)));
        assert!(matches!(r(b"\x01"), Ok(1)));
        assert!(matches!(r(b"\x7f"), Ok(0x7f)));
        assert!(matches!(r(b"\x7f"), Ok(127)));
        assert!(matches!(r(b"\x80\x01"), Ok(128)));
        assert!(matches!(r(b"\x80\x01"), Ok(128)));
        assert!(matches!(r(b"\xff\x01"), Ok(255)));
        assert!(matches!(r(b"\x80\x02"), Ok(256)));
    }

    #[test]
    fn var_i32() {
        let r = |x| Reader::new(HEADER, 0, x).var_i32();

        assert!(matches!(r(b"\x00"), Ok(0)));
        assert!(matches!(r(b"\x01"), Ok(-1)));
        assert!(matches!(r(b"\x02"), Ok(1)));
        assert!(matches!(r(b"\x03"), Ok(-2)));
        assert!(matches!(r(b"\x04"), Ok(2)));
    }

    #[test]
    fn str() {
        // Valid string
        let mut reader = Reader::new(HEADER, 0, b"hello\x00\x11");
        assert_eq!(reader.str().unwrap(), "hello");
        assert_eq!(reader.u8().unwrap(), 0x11);
        assert!(reader.is_empty());

        // Unterminated string
        let mut reader = Reader::new(HEADER, 0, b"hello");
        assert!(matches!(reader.str(), Err(Error::UnexpectedEof)));

        // Bad UTF-8
        let mut reader = Reader::new(HEADER, 0, b"\xc3\x28\x00");
        assert!(matches!(reader.str(), Err(Error::NonUtf8String)));
    }
}
