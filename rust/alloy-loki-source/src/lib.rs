use alloy_core::Registry;
use alloy_core::logs::Logs;

pub mod rustgen;

pub fn register(r: &mut Registry) {
    r.register_source::<Logs, rustgen::Rustgen>();
}
