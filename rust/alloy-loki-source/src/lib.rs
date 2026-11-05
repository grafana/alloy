use alloy_core::Registry;

pub mod rustgen;

pub fn register(r: &mut Registry) {
    r.register_source::<rustgen::Rustgen>();
}
