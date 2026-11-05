use alloy_core::Registry;
use alloy_core::logs::Logs;

pub mod rustecho;

pub fn register(r: &mut Registry) {
    r.register_sink::<Logs, rustecho::Rustecho>();
}
