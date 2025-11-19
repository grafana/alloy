// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

use super::*;

/// Result type shorthand.
pub type Result<T = (), E = Error> = std::result::Result<T, E>;

/// Errors that can occur during parsing.
#[non_exhaustive]
#[allow(missing_docs)]
#[derive(Debug, thiserror::Error)]
pub enum Error {
    #[error("Unable to find gopclntab")]
    GopclntabNotFound,

    #[error("Unable to find module data")]
    ModuleDataNotFound,

    #[error("Unable to find code section (`.text`)")]
    CodeSectionNotFound,

    #[error("Unable to resolve the destination of the gofunc pointer")]
    BadGoFuncPtr,

    #[error("Found pointer to invalid memory")]
    InvalidPtr,

    #[error("Go symbols section is malformed")]
    MalformedGopclntab,

    #[error("Go version is not supported")]
    UnsupportedGoVersion,

    #[error("Inline index is malformed")]
    BadInlineIndex,

    #[error("File index is malformed")]
    BadFileIndex,

    #[error("Line number is malformed")]
    BadLineNumber,

    #[error("Unexpected end of file")]
    UnexpectedEof,

    #[error("Encountered non-utf8 string")]
    NonUtf8String,

    #[error("Variable length integer is too big")]
    VarIntTooLong,

    #[error("Unable to read section without copying")]
    CannotAvoidCopy,

    #[error("Reader currently doesn't support big endian binaries")]
    BigEndian,

    #[error("objfile error: {}", .0)]
    Objfile(#[from] objfile::Error),
}
