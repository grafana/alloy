// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//! Minimal debug logging support.
//!
//! If we end up needing more elaborate logging later, it is worth considering
//! switching to the `log` crate and a corresponding subscriber. However, for
//! our current needs this seemed overkill.

use std::sync::atomic::AtomicBool;

// Re-export to make the macro show up in this module in rustdoc.
pub use crate::debug;

/// Determines whether [`debug`] messages are actually printed or not.
pub static ENABLED: AtomicBool = AtomicBool::new(false);

/// Print to stderr if debug printing is enabled.
///
/// See [`eprintln`] documentation for usage.
#[macro_export]
macro_rules! debug {
    ( $($args:tt)* ) => {
        if $crate::dbglog::ENABLED.load(::std::sync::atomic::Ordering::Relaxed) {
            ::std::eprintln!( $($args)* );
        }
    }
}
