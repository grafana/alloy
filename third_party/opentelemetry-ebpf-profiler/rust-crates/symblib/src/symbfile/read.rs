// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//! Implements a reader for the `symbfile` file format.

use super::*;
use crate::symbfile::proto::MessageType;
use crate::VirtAddr;
use fallible_iterator::FallibleIterator;
use std::io;

/// Reader for the `symbfile` file format.
#[derive(Debug)]
pub struct Reader<I: io::Read> {
    inner: I,
    read_buf: Vec<u8>,
    string_table: Vec<String>,
    prev_elf_va: Option<VirtAddr>,
}

/// Calculate the absolute ELF VA from context and updating instructions.
///
/// This is a macro because despite having exactly the same signature, the
/// `ElfVa` types in return pads and ranges are distinct auto-generated types,
/// so a regular function couldn't abstract over both of them.
macro_rules! elf_va_rel2abs {
    ($this:expr, $record:expr) => {{
        let abs = match $record.elf_va {
            Some(ElfVa::SetElfVa(abs)) => abs,
            Some(ElfVa::DeltaElfVa(rel)) => $this
                .prev_elf_va
                .ok_or(Error::RelativeValueWithoutReference)?
                .wrapping_add_signed(rel),
            None => return Err(Error::MissingRequiredField("elf_va")),
        };

        $this.prev_elf_va = Some(abs);

        abs
    }};
}

impl<I: io::Read> Reader<I> {
    /// Create a new reader.
    ///
    /// It's strongly advised to pass a buffered reader.
    pub fn new(mut inner: I) -> Result<Self> {
        // Check magic.
        let mut magic = [0u8; FILE_MAGIC.len()];
        inner.read_exact(&mut magic)?;
        if &magic != FILE_MAGIC {
            return Err(Error::InvalidMagic);
        }

        // Read and validate header.
        let mut read_buf = Vec::with_capacity(MSG_BUF_CAPACITY);
        let Some((kind, len)) = read_msg_prefix(&mut inner)? else {
            return Err(Error::TruncatedMessage);
        };

        if kind != Some(MessageType::MtHeader) {
            return Err(Error::UnexpectedType {
                expected: MessageType::MtHeader,
                actual: kind,
            });
        }

        let _: proto::Header = read_msg(&mut inner, len, &mut read_buf)?;

        Ok(Self {
            inner,
            read_buf,
            string_table: Vec::new(),
            prev_elf_va: None,
        })
    }

    /// Read the next record from the symbfile.
    pub fn read(&mut self) -> Result<Option<Record>> {
        loop {
            let Some((kind, len)) = read_msg_prefix(&mut self.inner)? else {
                return Ok(None);
            };

            match kind {
                Some(MessageType::MtInvalid | MessageType::MtHeader) => {
                    return Err(Error::InvalidMessageType);
                }

                Some(MessageType::MtRangeV1) => {
                    let raw: proto::RangeV1 = read_msg(&mut self.inner, len, &mut self.read_buf)?;
                    let parsed = self.deserialize_range(raw)?;
                    return Ok(Some(Record::Range(parsed)));
                }

                Some(MessageType::MtReturnPadV1) => {
                    let raw: proto::ReturnPadV1 =
                        read_msg(&mut self.inner, len, &mut self.read_buf)?;
                    let parsed = self.deserialize_return_pad(raw)?;
                    return Ok(Some(Record::ReturnPad(parsed)));
                }

                Some(MessageType::MtStringTableV1) => {
                    let msg: proto::StringTableV1 =
                        read_msg(&mut self.inner, len, &mut self.read_buf)?;
                    self.string_table = msg.strings;
                }

                // Skip unsupported messages.
                #[allow(unreachable_patterns)]
                Some(_) | None => continue,
            }
        }
    }

    /// Convert a range in wire format into our higher-level format.
    pub fn deserialize_range(&mut self, range: proto::RangeV1) -> Result<Range> {
        let line_table = if let Some(lt) = range.line_table {
            if lt.offset.len() != lt.line_number.len() {
                return Err(Error::ColumnLengthMismatch);
            }

            let mut prev_offset = 0;
            lt.line_number
                .into_iter()
                .zip(lt.offset)
                .map(|(l, o)| {
                    let offset = prev_offset + o;
                    prev_offset += o;

                    LineTableEntry {
                        offset,
                        line_number: l,
                    }
                })
                .collect()
        } else {
            Default::default()
        };

        use proto::range_v1::{CallFile, ElfVa, File, Func};
        Ok(Range {
            elf_va: elf_va_rel2abs!(self, range),
            length: range.length as u32,
            func: match range.func {
                Some(Func::FuncRef(idx)) => self.str_by_idx(idx)?.to_owned(),
                Some(Func::FuncStr(s)) => s,
                None => return Err(Error::MissingRequiredField("func")),
            },
            file: match range.file {
                Some(File::FileRef(idx)) => Some(self.str_by_idx(idx)?.to_owned()),
                Some(File::FileStr(s)) => Some(s),
                None => None,
            },
            call_file: match range.call_file {
                Some(CallFile::CallFileRef(idx)) => Some(self.str_by_idx(idx)?.to_owned()),
                Some(CallFile::CallFileStr(s)) => Some(s),
                None => None,
            },
            call_line: if range.call_line == 0 {
                None
            } else {
                Some(range.call_line)
            },
            depth: range.depth,
            line_table,
        })
    }

    /// Convert a return pad in wire format into our higher-level format.
    fn deserialize_return_pad(&mut self, pad: proto::ReturnPadV1) -> Result<ReturnPad> {
        if pad.file.len() != pad.func.len() || pad.file.len() != pad.line.len() {
            return Err(Error::ColumnLengthMismatch);
        }

        use proto::return_pad_v1::ElfVa;
        Ok(ReturnPad {
            elf_va: elf_va_rel2abs!(self, pad),
            entries: pad
                .file
                .into_iter()
                .zip(pad.func)
                .zip(pad.line)
                .map(|((file, func), line)| {
                    let file = self.str_by_idx(file)?.to_owned();

                    Ok(ReturnPadEntry {
                        func: self.str_by_idx(func)?.to_owned(),
                        file: if file.is_empty() { None } else { Some(file) },
                        line: if line == 0 { None } else { Some(line) },
                    })
                })
                .collect::<Result<_, Error>>()?,
        })
    }

    /// Retrieve the given string table entry via its index.
    fn str_by_idx(&self, idx: u32) -> Result<&str> {
        let s = self
            .string_table
            .get(idx as usize)
            .ok_or(Error::InvalidStringTableIndex)?
            .as_str();
        Ok(s)
    }
}

/// Allow using the reader as an iterator.
impl<I: io::Read> FallibleIterator for Reader<I> {
    type Item = Record;
    type Error = Error;

    fn next(&mut self) -> Result<Option<Self::Item>, Self::Error> {
        self.read()
    }
}

/// Reads the var-int encoded message length and type.
fn read_msg_prefix(mut read: impl io::Read) -> Result<Option<(Option<MessageType>, u32)>> {
    let Some(length) = read_leb128(&mut read)? else {
        // EOF is fine here: the file ended after the previous message.
        return Ok(None);
    };

    if length > u64::from(MAX_MSG_SIZE) {
        return Err(Error::MaximumMsgSizeExceeded(length));
    }

    let Some(raw_kind) = read_leb128(&mut read)? else {
        return Err(Error::TruncatedMessage);
    };

    let kind: i32 = raw_kind.try_into().map_err(|_| Error::InvalidMessageType)?;

    Ok(Some((MessageType::try_from(kind).ok(), length as u32)))
}

/// Reads a protobuf message from the input stream,
/// using `buf` as a temporary buffer for decoding.
fn read_msg<M: prost::Message + Default>(
    mut read: impl io::Read,
    length: u32,
    buf: &mut Vec<u8>,
) -> Result<M> {
    buf.resize(length as usize, 0);

    match read.read_exact(&mut buf[..]) {
        Ok(_) => (),
        Err(e) if e.kind() == io::ErrorKind::UnexpectedEof => return Err(Error::TruncatedMessage),
        Err(e) => return Err(Error::IO(e)),
    };

    buf.shrink_to(MSG_BUF_CAPACITY);
    Ok(M::decode(&buf[..])?)
}

/// Read an ULEB-128 encoded variable-length integer.
///
/// If EOF is reached before reading the first byte, `Ok(None)` is returned.
/// If EOF is encountered in the middle of an incomplete var-int sequence,
/// a corresponding IO error is returned.
fn read_leb128(mut read: impl io::Read) -> Result<Option<u64>> {
    let mut result = 0;
    let mut shift = 0;
    let mut buf = [0u8];

    for i in 0..10 {
        match read.read_exact(&mut buf) {
            Ok(_) => (),
            Err(e) if i == 0 && e.kind() == io::ErrorKind::UnexpectedEof => return Ok(None),
            Err(e) => return Err(Error::IO(e)),
        }

        result |= ((buf[0] & 0x7F) as u64) << shift;

        if buf[0] & 0x80 == 0 {
            return Ok(Some(result));
        }

        shift += 7;
    }

    Err(Error::VarIntTooLong)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn leb128() {
        // Test cases ported from Tim's Go implementation.
        assert!(matches!(
            read_leb128(&[0xE5, 0x8E, 0xA6][..]),
            Err(Error::IO(e)) if e.kind() == io::ErrorKind::UnexpectedEof,
        ));
        assert!(matches!(read_leb128(&[][..]), Ok(None),));
        assert!(matches!(
            read_leb128(&[0x95, 0x9a, 0xef, 0x3a][..]),
            Ok(Some(123456789)),
        ));
        assert!(matches!(
            read_leb128(&[0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0x01][..]),
            Ok(Some(u64::MAX)),
        ));
        assert!(matches!(read_leb128(&[0x00][..]), Ok(Some(0))));
        assert!(matches!(read_leb128(&[0x01][..]), Ok(Some(1))));
        assert!(matches!(read_leb128(&[0x7f][..]), Ok(Some(0x7f))));
        assert!(matches!(read_leb128(&[0x7f][..]), Ok(Some(127))));
        assert!(matches!(read_leb128(&[0x80, 0x01][..]), Ok(Some(128))));
        assert!(matches!(read_leb128(&[0x80, 0x01][..]), Ok(Some(128))));
        assert!(matches!(read_leb128(&[0xff, 0x01][..]), Ok(Some(255))));
        assert!(matches!(read_leb128(&[0x80, 0x02][..]), Ok(Some(256))));
    }
}
