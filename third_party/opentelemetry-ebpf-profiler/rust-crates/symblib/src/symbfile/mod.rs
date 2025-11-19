// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//! Implements the `symbfile` file format.
//!
//! For documentation about the file format, please refer to `proto/symbfile/symbfile.proto`.

pub mod proto;
pub mod read;
pub mod records;
mod strdedup;
pub mod write;

// Re-export core types on the main module.
pub use read::Reader;
pub use records::*;
pub use write::Writer;

/// Magic that every valid symbfile must start with.
const FILE_MAGIC: &[u8; 8] = b"symbfile";

/// Size of the persistent read/write buffer for protobuf messages.
const MSG_BUF_CAPACITY: usize = 4096;

/// Maximum size of an individual message.
const MAX_MSG_SIZE: u32 = 16 * 1024 * 1024; // 16 MiB

/// Maximum size of the string table before flushing it.
///
/// Must be smaller than [`MAX_MSG_SIZE`].
const STRING_TABLE_SIZE_FLUSH_THRESH: u32 = MAX_MSG_SIZE - 64 * 1024;

/// Maximum size of the internal message buffer in the writer.
///
/// This impacts how many messages can use the same string table before
/// being written out.
const WRITER_MSG_BUFFER_SIZE: usize = 64 * 1024 * 1024; // 64 MiB

/// Result type used throughout this module.
pub type Result<T = (), E = Error> = std::result::Result<T, E>;

/// Errors that can occur when reading or writing `symbfile`s.
#[non_exhaustive]
#[allow(missing_docs)]
#[derive(Debug, thiserror::Error)]
pub enum Error {
    #[error("Line-table is not sorted by `offset`")]
    LineTableNotSorted,

    #[error("File does not start with the expected magic")]
    InvalidMagic,

    #[error("Expected message type {:?} but got {:?}", .expected, .actual)]
    UnexpectedType {
        expected: proto::MessageType,
        // None = not known by our protobuf definition
        actual: Option<proto::MessageType>,
    },

    #[error("File kind with ID {} is unsupported by this implementation", .0)]
    UnsupportedKind(i32),

    #[error("Message type value is invalid")]
    InvalidMessageType,

    #[error("Message contains an invalid string table reference")]
    InvalidStringTableIndex,

    #[error("Message is missing a required field")]
    MissingRequiredField(&'static str),

    #[error("File ended prematurely in the middle of a message")]
    TruncatedMessage,

    #[error("Variable-length integer is too big")]
    VarIntTooLong,

    #[error("Message of size {} exceeds maximum of {}", .0, MAX_MSG_SIZE)]
    MaximumMsgSizeExceeded(u64),

    #[error("Not all arrays in a columnar struct-of-arrays have the same length")]
    ColumnLengthMismatch,

    #[error("Encountered relative value without an absolut value preceding it")]
    RelativeValueWithoutReference,

    #[error("IO error")]
    IO(#[from] std::io::Error),

    #[error("Encoding error")]
    Encoding(#[from] prost::EncodeError),

    #[error("Decoding error")]
    Decoding(#[from] prost::DecodeError),
}

#[cfg(test)]
mod tests {
    use super::*;

    fn make_test_range(seed: u64) -> Range {
        Range {
            elf_va: seed * 12,
            length: (seed % 1234) as u32,
            func: match seed % 2 {
                0 => "main".to_owned(),
                1 => "strlen".to_owned(),
                _ => unreachable!(),
            },
            file: match seed % 3 {
                0 => Some("main.c".to_owned()),
                1 => None,
                2 => Some("/usr/libc/slen.c".to_owned().to_owned()),
                _ => unreachable!(),
            },
            call_file: match seed % 3 {
                0 => None,
                1 => Some("foo.h".to_owned()),
                2 => Some("bar.c".to_owned()),
                _ => unreachable!(),
            },
            call_line: match seed % 133 {
                11 | 22 | 33 => None,
                other => Some(other as u32 + 1),
            },
            depth: (seed % 3) as u32,
            line_table: (0..seed % 31)
                .map(|i| LineTableEntry {
                    offset: (seed + i * (seed % 3)) as u32,
                    line_number: (i * (seed % 7) + 1) as u32,
                })
                .collect(),
        }
    }

    fn make_test_retpad(seed: u64) -> ReturnPad {
        ReturnPad {
            elf_va: (0x130 * seed) as u64,
            entries: (0..seed % 31 + 1)
                .map(|i| {
                    let file = match (seed ^ i) % 3 {
                        0 => "main.c",
                        1 => "hello.cc",
                        2 => "blah.go",
                        _ => unreachable!(),
                    };

                    let func = match (seed ^ i) % 4 {
                        0 => "main",
                        1 => "print_hello",
                        2 => "handle_error",
                        3 => "do_something",
                        _ => unreachable!(),
                    };

                    ReturnPadEntry {
                        func: func.to_owned(),
                        file: Some(file.to_owned()),
                        line: Some((i * (seed % 7)) as u32 + 1),
                    }
                })
                .collect(),
        }
    }

    #[test]
    fn round_trip() {
        let msgs: Vec<_> = (0..1000)
            .map(|i| {
                if (!i) % 3 > 0 {
                    Record::Range(make_test_range(i))
                } else {
                    Record::ReturnPad(make_test_retpad(i))
                }
            })
            .collect();

        let mut writer = Writer::new(Vec::new()).unwrap();
        for msg in &msgs {
            writer.write(msg.clone()).unwrap();
        }
        let buf = writer.finalize().unwrap();

        let mut reader = Reader::new(&buf[..]).unwrap();
        let mut expected_iter = msgs.iter();
        while let Some(msg) = reader.read().unwrap() {
            let expected = expected_iter.next().unwrap();
            assert_eq!(&msg, expected);
        }
    }
}
