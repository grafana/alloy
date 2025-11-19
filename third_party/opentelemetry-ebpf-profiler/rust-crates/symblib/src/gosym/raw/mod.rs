// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//! Provides decoding of the raw data structures.

mod reader;
mod regions;
mod structs;
mod types;

// Re-export some stuff that is needed across all `raw` submodules.
use super::{Error, Result, Version};
use crate::VirtAddr;

pub use reader::*;
pub use regions::*;
pub use structs::*;
pub use types::*;
