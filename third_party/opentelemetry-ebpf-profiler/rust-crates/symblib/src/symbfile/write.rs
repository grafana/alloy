// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//! Implements a writer for the `symbfile` file format.

use super::*;
use crate::symbfile::proto::MessageType;
use crate::VirtAddr;
use std::{io, mem};

/// Writer for the `symbfile` file format.
#[derive(Debug)]
pub struct Writer<O: io::Write> {
    out: O,
    write_buf: Vec<u8>,
    string_table: strdedup::Builder,
    buffered_msgs: Vec<BufferedMsg>,
    prev_elf_va: Option<VirtAddr>,
}

/// Calculate either a relative or absolute ELF VA update.
///
/// This is a macro because despite having exactly the same signature, the
/// `ElfVa` types in return pads and ranges are distinct auto-generated types,
/// so a regular function couldn't abstract over both of them.
macro_rules! elf_va_abs2rel {
    ($this:expr, $record:expr) => {{
        match mem::replace(&mut $this.prev_elf_va, Some($record.elf_va)) {
            Some(prev) => {
                let i128_delta = $record.elf_va as i128 - prev as i128;
                let maybe_i64_delta: Result<i64, _> = i128_delta.try_into();

                // Jumps further away than 2^63 cannot be expressed as i64:
                // use absolute updates to represent such cases.
                if let Ok(delta) = maybe_i64_delta {
                    Some(ElfVa::DeltaElfVa(delta))
                } else {
                    Some(ElfVa::SetElfVa($record.elf_va))
                }
            }
            None => Some(ElfVa::SetElfVa($record.elf_va)),
        }
    }};
}

impl<O: io::Write> Writer<O> {
    /// Create a new writer that outputs into `out`.
    pub fn new(out: O) -> Result<Self> {
        let mut writer = Writer {
            out,
            write_buf: Vec::with_capacity(MSG_BUF_CAPACITY),
            string_table: strdedup::Builder::default(),
            buffered_msgs: Vec::new(),
            prev_elf_va: None,
        };

        writer.out.write_all(FILE_MAGIC)?;
        writer.write_msg(MessageType::MtHeader, proto::Header {})?;

        Ok(writer)
    }

    /// Write a record to the file.
    pub fn write(&mut self, record: impl Into<Record>) -> Result {
        let msg = match record.into() {
            Record::Range(record) => BufferedMsg::Range(self.serialize_range(record)?),
            Record::ReturnPad(pad) => BufferedMsg::ReturnPad(self.serialize_return_pad(pad)?),
        };

        self.buffered_msgs.push(msg);

        if self.string_table.size_estimate() >= STRING_TABLE_SIZE_FLUSH_THRESH as usize
            || self.buffered_msgs.len() * mem::size_of::<BufferedMsg>() >= WRITER_MSG_BUFFER_SIZE
        {
            self.flush_buffered_msgs()?;
        }

        Ok(())
    }

    /// Convert a high-level range to the wire format.
    fn serialize_range(&mut self, range: Range) -> Result<proto::RangeV1> {
        use proto::range_v1::{CallFile, ElfVa, File, Func};

        let line_table = if range.line_table.is_empty() {
            None
        } else {
            let mut columnar = proto::LineTable {
                offset: Vec::with_capacity(range.line_table.len()),
                line_number: Vec::with_capacity(range.line_table.len()),
            };

            let mut prev_offset = 0;
            for row in range.line_table {
                let offset_delta = row
                    .offset
                    .checked_sub(prev_offset)
                    .ok_or(Error::LineTableNotSorted)?;

                columnar.offset.push(offset_delta);
                columnar.line_number.push(row.line_number);

                prev_offset = row.offset;
            }

            debug_assert_eq!(columnar.offset.len(), columnar.line_number.len());

            Some(columnar)
        };

        // Transform the remaining data.
        Ok(proto::RangeV1 {
            elf_va: elf_va_abs2rel!(self, range),
            length: u64::from(range.length),
            func: Some(Func::FuncRef(self.string_table.index_for_str(range.func))),
            file: range
                .file
                .map(|x| File::FileRef(self.string_table.index_for_str(x))),
            call_line: range.call_line.unwrap_or(0),
            call_file: range
                .call_file
                .map(|x| CallFile::CallFileRef(self.string_table.index_for_str(x))),
            depth: range.depth,
            line_table,
        })
    }

    /// Convert a high-level return pad to the wire format.
    fn serialize_return_pad(&mut self, pad: ReturnPad) -> Result<proto::ReturnPadV1> {
        use proto::return_pad_v1::ElfVa;
        let mut return_pads = proto::ReturnPadV1 {
            elf_va: elf_va_abs2rel!(self, pad),
            func: Vec::with_capacity(pad.entries.len()),
            file: Vec::with_capacity(pad.entries.len()),
            line: Vec::with_capacity(pad.entries.len()),
        };

        for entry in pad.entries {
            return_pads
                .func
                .push(self.string_table.index_for_str(entry.func));
            return_pads.file.push(
                self.string_table
                    .index_for_str(entry.file.unwrap_or_default()),
            );
            return_pads.line.push(entry.line.unwrap_or_default());
        }

        Ok(return_pads)
    }

    /// Write out and clear the buffered messages and string table.
    fn flush_buffered_msgs(&mut self) -> Result {
        let mut translator = mem::take(&mut self.string_table).build();
        let mut msgs = mem::take(&mut self.buffered_msgs);

        // Fixing up the messages can still mutate the final string table, so
        // we have to do two passes through the array for fixup and sending.
        for buffered_msg in &mut msgs {
            match buffered_msg {
                BufferedMsg::Range(range) => Self::fix_up_range(&mut translator, range),
                BufferedMsg::ReturnPad(pad) => Self::fix_up_return_pad(&mut translator, pad),
            };
        }

        self.write_msg(
            MessageType::MtStringTableV1,
            proto::StringTableV1 {
                strings: translator.into_table(),
            },
        )?;

        for buffered_msg in msgs {
            match buffered_msg {
                BufferedMsg::Range(range) => self.write_msg(MessageType::MtRangeV1, range)?,
                BufferedMsg::ReturnPad(pad) => self.write_msg(MessageType::MtReturnPadV1, pad)?,
            };
        }

        Ok(())
    }

    fn fix_up_range(trans: &mut strdedup::Mapper, range: &mut proto::RangeV1) {
        use proto::range_v1::{CallFile, File, Func};
        use strdedup::Mapping::*;

        let Some(Func::FuncRef(func_idx)) = &mut range.func else {
            unreachable!("bug: non index func field")
        };
        range.func = match trans.translate(*func_idx) {
            Unique(x) => Some(Func::FuncStr(x)),
            Translate(x) => Some(Func::FuncRef(x)),
        };

        if let Some(file) = &mut range.file {
            let File::FileRef(file_idx) = file else {
                unreachable!("bug: non index file field")
            };
            *file = match trans.translate(*file_idx) {
                Unique(x) => File::FileStr(x),
                Translate(x) => File::FileRef(x),
            };
        }

        if let Some(call_file) = &mut range.call_file {
            let CallFile::CallFileRef(call_file_idx) = call_file else {
                unreachable!("bug: non index call_file field")
            };
            *call_file = match trans.translate(*call_file_idx) {
                Unique(x) => CallFile::CallFileStr(x),
                Translate(x) => CallFile::CallFileRef(x),
            };
        }
    }

    fn fix_up_return_pad(trans: &mut strdedup::Mapper, pad: &mut proto::ReturnPadV1) {
        for field in [&mut pad.func, &mut pad.file] {
            for item in field {
                *item = trans.force_entry(*item);
            }
        }
    }

    /// Finalize the file, flushing all remaining buffers.
    ///
    /// Returns the output stream once all buffers are flushed.
    pub fn finalize(mut self) -> Result<O> {
        self.flush_buffered_msgs()?;
        self.out.flush()?;
        Ok(self.out)
    }

    /// Gets an immutable reference to the underlying stream.
    pub fn stream_ref(&self) -> &O {
        &self.out
    }

    /// Write the given message to the output stream.
    ///
    /// `write_buf` is used as a temporary buffer to avoid unnecessarily
    /// allocating and freeing on every call.
    fn write_msg(&mut self, kind: MessageType, msg: impl prost::Message) -> Result {
        self.write_buf.clear();

        let encoded_len = msg.encoded_len();
        if encoded_len > MAX_MSG_SIZE as usize {
            return Err(Error::MaximumMsgSizeExceeded(encoded_len as u64));
        }

        prost::encode_length_delimiter(encoded_len, &mut self.write_buf)?;
        prost::encode_length_delimiter(kind as usize, &mut self.write_buf)?;
        msg.encode(&mut self.write_buf)?;
        self.out.write_all(&self.write_buf)?;

        // Make sure the write buffer doesn't stay huge if one message was big.
        self.write_buf.shrink_to(MSG_BUF_CAPACITY);

        Ok(())
    }
}

#[derive(Debug)]
enum BufferedMsg {
    Range(proto::RangeV1),
    ReturnPad(proto::ReturnPadV1),
}
