// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//! Minimal instruction disassembler implementation.

use crate::{AnyError, VirtAddr};
use fallible_iterator::FallibleIterator;
use std::iter;

/// Errors that can occur during instruction decoding.
#[non_exhaustive]
#[allow(missing_docs)]
#[derive(Debug, thiserror::Error)]
pub enum Error {
    #[error("Not enough bytes to decode the full instruction at {:#08X}", .0)]
    TruncatedInstruction(VirtAddr),

    #[error("Bytes at {:#08X} do not form a valid instruction", .0)]
    InvalidInstruction(VirtAddr),

    #[error(transparent)]
    Other(AnyError),
}

/// Trait for instruction decoders.
pub trait InstrDecoder {
    /// Decode one instruction and return information.
    fn decode(&self, addr: VirtAddr, buf: &[u8]) -> Result<InstrInfo, Error>;
}

/// Information about an instruction.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct InstrInfo {
    /// Virtual addresss of the instruction.
    pub addr: VirtAddr,

    /// Whether the instruction is a call or syscall.
    pub is_call: bool,

    /// Length of the instruction, in bytes.
    pub length: u8,
}

/// AMD64 (X86-64) instruction decoder.
///
/// Currently implemented using the Zydis library.
#[derive(Debug)]
pub struct Amd64InstrDecoder(zydis::Decoder);

impl Default for Amd64InstrDecoder {
    fn default() -> Self {
        Self(zydis::Decoder::new64())
    }
}

impl InstrDecoder for Amd64InstrDecoder {
    fn decode(&self, addr: VirtAddr, buf: &[u8]) -> Result<InstrInfo, Error> {
        use zydis::{Mnemonic as M, ZYAN_MODULE_ZYDIS};

        let insn = match self.0.decode_first::<zydis::NoOperands>(buf) {
            Ok(Some(insn)) => insn,
            Ok(None) => return Err(Error::TruncatedInstruction(addr)),
            Err(e) if e.module() == ZYAN_MODULE_ZYDIS => {
                return Err(Error::InvalidInstruction(addr))
            }
            Err(e) => return Err(Error::Other(Box::new(e))),
        };

        Ok(InstrInfo {
            addr,
            is_call: matches!(insn.mnemonic, M::CALL | M::SYSCALL | M::INT | M::INTO),
            length: insn.length,
        })
    }
}

/// ARM64 (aarch64) instruction decoder.
///
/// Currently a hand-rolled minimal decoder.
#[derive(Debug, Default)]
pub struct Aarch64InstrDecoder;

impl InstrDecoder for Aarch64InstrDecoder {
    fn decode(&self, addr: VirtAddr, buf: &[u8]) -> Result<InstrInfo, Error> {
        // For the few instructions that we care about, it's simple enough to
        // do the decoding ourselves, so that's what we're doing here.

        const BL: u32 = 0b10010100000000000000000000000000;
        const BL_MASK: u32 = 0b11111100000000000000000000000000;
        const BLR: u32 = 0b11010110001111110000000000000000;
        const BLR_MASK: u32 = 0b11111111111111111111110000011111;
        const SVC: u32 = 0b11010100000000000000000000000001;
        const SVC_MASK: u32 = 0b11111111111000000000000000011111;

        if buf.len() < 4 {
            return Err(Error::TruncatedInstruction(addr));
        }

        let insn = u32::from_le_bytes(buf[..4].try_into().unwrap());
        let is_bl = (insn & BL_MASK) == BL;
        let is_blr = (insn & BLR_MASK) == BLR;
        let is_svc = (insn & SVC_MASK) == SVC;

        Ok(InstrInfo {
            addr,
            is_call: is_bl || is_blr || is_svc,
            length: 4,
        })
    }
}

/// Creates an iterator decoding all instructions in the given buffer.
pub fn decode_all<'a, D: InstrDecoder + ?Sized + 'a>(
    decoder: &'a D,
    mut addr: VirtAddr,
    mut buf: &'a [u8],
) -> impl FallibleIterator<Item = InstrInfo, Error = Error> + 'a {
    fallible_iterator::convert(iter::from_fn(move || {
        if buf.is_empty() {
            return None;
        }

        let result = decoder.decode(addr, buf);

        if let Ok(insn) = &result {
            buf = &buf[insn.length as usize..];
            addr += insn.length as VirtAddr;
        }

        Some(result)
    }))
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn dec_all() {
        let dec = Amd64InstrDecoder::default();
        let mut iter = decode_all(&dec, 0x10, b"\xCC\x90\xE8\x11\x22\x33\x44");
        assert_eq!(
            iter.next().unwrap(),
            Some(InstrInfo {
                is_call: false,
                length: 1,
                addr: 0x10,
            })
        );
        assert_eq!(
            iter.next().unwrap(),
            Some(InstrInfo {
                is_call: false,
                length: 1,
                addr: 0x11,
            })
        );
        assert_eq!(
            iter.next().unwrap(),
            Some(InstrInfo {
                is_call: true,
                length: 5,
                addr: 0x12,
            })
        );
        assert_eq!(iter.next().unwrap(), None);
    }

    #[test]
    fn amd64() {
        let dec = Amd64InstrDecoder::default();
        assert!(matches!(
            dec.decode(0, b""),
            Err(Error::TruncatedInstruction(0))
        ));
        assert!(matches!(
            dec.decode(0, b"\xE9"),
            Err(Error::TruncatedInstruction(0))
        ));
        assert_eq!(
            dec.decode(0, b"\xE9\x00\x00\x00\x00").unwrap(),
            InstrInfo {
                addr: 0,
                length: 5,
                is_call: false
            }
        );
        assert_eq!(
            dec.decode(123, b"\xE8\x00\x00\x00\x00").unwrap(),
            InstrInfo {
                addr: 123,
                length: 5,
                is_call: true,
            }
        );
    }

    #[test]
    fn aarch64() {
        let dec = Aarch64InstrDecoder::default();
        assert!(matches!(
            dec.decode(0, b""),
            Err(Error::TruncatedInstruction(0))
        ));
        assert!(matches!(
            dec.decode(0, b"\xAA"),
            Err(Error::TruncatedInstruction(0))
        ));
        assert_eq!(
            dec.decode(33, b"\x00\x00\x3f\xd6").unwrap(),
            InstrInfo {
                addr: 33,
                is_call: true,
                length: 4,
            }
        );
        assert_eq!(
            dec.decode(44, b"\x8d\x04\x00\x94").unwrap(),
            InstrInfo {
                addr: 44,
                is_call: true,
                length: 4,
            }
        );
        assert_eq!(
            dec.decode(0, b"\x8d\x04\x00\x14").unwrap(),
            InstrInfo {
                addr: 0,
                is_call: false,
                length: 4,
            }
        );
        assert_eq!(
            dec.decode(0x123444, b"\x1f\x20\x03\xd5").unwrap(),
            InstrInfo {
                addr: 0x123444,
                is_call: false,
                length: 4,
            }
        );
    }
}
