//! Describes components so their Go side can be generated.

use std::time::Duration;

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum Kind {
    Source,
    Sink,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum Stability {
    Experimental,
    PublicPreview,
    GenerallyAvailable,
}

#[derive(Debug)]
pub struct ComponentInfo {
    pub name: &'static str,
    pub kind: Kind,
    pub stability: Stability,
    pub args: &'static [Field],
}

/// An argument of a component. The fields of `Arguments::schema` must be in
/// the same order as the fields of `Arguments::Raw`.
#[derive(Debug)]
pub struct Field {
    /// The attribute name in the Alloy config.
    pub name: &'static str,
    pub ty: FieldType,
    /// Required if `None`.
    pub default: Option<Value>,
    pub min: Option<Value>,
    pub max: Option<Value>,
    pub doc: &'static str,
}

/// How a field is passed in `Raw`.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum FieldType {
    U8,
    U16,
    U32,
    U64,
    I8,
    I16,
    I32,
    I64,
    F32,
    F64,
    Bool,
    /// `u64` milliseconds.
    DurationMs,
}

#[derive(Debug, Clone, Copy, PartialEq)]
pub enum Value {
    Uint(u64),
    Int(i64),
    Float(f64),
    Bool(bool),
    Duration(Duration),
}
