use alloy_core::Registry;
use alloy_core::schema::Stability;

pub mod rustgen;

pub fn register(r: &mut Registry) {
    r.register_source::<rustgen::Rustgen>("loki.source.rustgen", Stability::Experimental);
}
