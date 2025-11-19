// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//! Types for uniquely identifying files.

use base64::Engine;
use sha2::digest::FixedOutput;
use sha2::Digest as _;
use std::io::Read as _;
use std::{fmt, fs, io, path};

/// Size of the head and tail blocks used for partially hashing ELF files.
const PARTIAL_HASH_SIZE: u64 = 4096;

/// Hash-based unique file identifier.
///
/// This ID is compatible with the file ID format that is calculated and sent
/// to the collection agent by our host agent.
///
/// https://github.com/open-telemetry/opentelemetry-ebpf-profiler/blob/cd3963/libpf/fileid.go#L124
#[repr(transparent)]
#[derive(PartialEq, Eq, Hash, Clone, Copy)]
pub struct FileId(u128);

impl FileId {
    /// Construct the ID from two  `u64`  halves.
    pub fn from_parts(hi: u64, lo: u64) -> Self {
        Self((hi as u128) << 64 | lo as u128)
    }

    /// Calculates the file ID for the file at the given path.
    pub fn from_path(path: &path::Path) -> io::Result<Self> {
        Self::from_stream(&fs::File::open(path)?)
    }

    /// Calculates the file ID from the given seekable stream.
    ///
    /// If this function succeeds, the stream is seeked back to the original
    /// position, otherwise the file position is undefined.
    pub fn from_stream(mut stream: impl io::Read + io::Seek) -> io::Result<Self> {
        let prev_pos = stream.seek(io::SeekFrom::End(0))?;
        let stream_len = stream.stream_position()?;
        let mut hasher = sha2::Sha256::new();

        // Hash first 4096 bytes.
        stream.seek(io::SeekFrom::Start(0))?;
        io::copy(&mut stream.by_ref().take(PARTIAL_HASH_SIZE), &mut hasher)?;

        // Hash last 4096 bytes.
        let tail_start = stream_len.saturating_sub(PARTIAL_HASH_SIZE);
        stream.seek(io::SeekFrom::Start(tail_start))?;
        io::copy(&mut stream.by_ref().take(PARTIAL_HASH_SIZE), &mut hasher)?;

        // Hash length.
        hasher.update(u64::to_be_bytes(stream_len));

        stream.seek(io::SeekFrom::Start(prev_pos))?;

        let digest: [u8; 32] = hasher.finalize_fixed().into();
        let truncated: [u8; 16] = digest[..16].try_into().unwrap();
        Ok(Self(u128::from_be_bytes(truncated)))
    }

    /// Formats the ID in the base64 format used in our ES indices.
    pub fn format_es(&self) -> String {
        let mut out = String::with_capacity(128);
        ES_B64_ENGINE.encode_string(self.0.to_be_bytes(), &mut out);
        out
    }

    /// Try to parse a file ID in ES format.
    ///
    /// Returns `None` if the input is not a valid file ID.
    pub fn try_parse_es(text_repr: &str) -> Option<FileId> {
        let bytes = ES_B64_ENGINE.decode(text_repr).ok()?;
        let sized: [u8; 16] = bytes.try_into().ok()?;
        Some(Self(u128::from_be_bytes(sized)))
    }

    /// Formats the ID as a lower-case hexadecimal number.
    pub fn format_hex(&self) -> String {
        format!("{:032x}", self.0)
    }

    /// Try to parse a file ID from a string in hex format.
    pub fn try_parse_hex(text_repr: &str) -> Option<FileId> {
        let tmp = u128::from_str_radix(text_repr, 16).ok()?;
        Some(Self(tmp))
    }
}

/// base64 engine that en-/decodes in our ES base64 representation.
static ES_B64_ENGINE: base64::engine::GeneralPurpose = base64::engine::GeneralPurpose::new(
    &base64::alphabet::URL_SAFE,
    base64::engine::GeneralPurposeConfig::new()
        .with_encode_padding(false)
        .with_decode_padding_mode(base64::engine::DecodePaddingMode::Indifferent),
);

/// Debug formatting.
impl fmt::Debug for FileId {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "FileId({})", self.format_hex())
    }
}

/// Construct a file ID from an unsigned 128 bit integer.
impl From<u128> for FileId {
    fn from(value: u128) -> Self {
        Self(value)
    }
}

/// Get a file ID as a 128 bit integer.
impl From<FileId> for u128 {
    fn from(value: FileId) -> Self {
        value.0
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::tests::testdata;

    #[test]
    fn hash_elf() {
        let id = FileId::from_path(&testdata("inline")).unwrap();
        assert_eq!(id, 0xc34f3585fca1b579fb458e827851a599.into());
        assert_eq!(id.format_hex(), "c34f3585fca1b579fb458e827851a599");
        assert_eq!(id.format_es(), "w081hfyhtXn7RY6CeFGlmQ");
    }

    #[test]
    fn hash_non_elf() {
        let zeros = &[0u8; 123];
        let mut cursor = io::Cursor::new(&zeros);
        let id = FileId::from_stream(&mut cursor).unwrap();
        assert_eq!(id, 0xf4c1e5fe2f28034fcceb0776ec00b125.into());
        assert_eq!(id.format_hex(), "f4c1e5fe2f28034fcceb0776ec00b125");
        assert_eq!(id.format_es(), "9MHl_i8oA0_M6wd27ACxJQ");
    }
}
