use alloy_core::Registry;

pub mod rustecho;

pub fn register(r: &mut Registry) {
    r.register_sink::<rustecho::Rustecho>();
}
