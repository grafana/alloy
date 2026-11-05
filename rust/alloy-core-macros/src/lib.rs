use proc_macro::TokenStream;
use proc_macro2::{Span, TokenStream as TokenStream2};
use quote::quote;
use syn::{Expr, ExprLit, ExprUnary, Fields, ItemStruct, Lit, Type, UnOp, parse_macro_input};

/// Turns a struct into component arguments: derives serde's `Deserialize`
/// (rejecting unknown fields) and implements `alloy_core::Arguments` with the
/// struct's schema.
///
/// Fields can be integers, floats, `bool`, `Duration`, `String` or a
/// `BTreeMap`/`HashMap` of `String` to `String`. Doc comments become the
/// field's doc, and `#[arg(default = .., min = .., max = ..)]` sets the rest of
/// its schema. Durations take strings like `"1s"` or `"500ms"`. Fields without
/// a default are required.
#[proc_macro_attribute]
pub fn arguments(attr: TokenStream, item: TokenStream) -> TokenStream {
    if !attr.is_empty() {
        return syn::Error::new(Span::call_site(), "arguments takes no parameters")
            .to_compile_error()
            .into();
    }

    let item = parse_macro_input!(item as ItemStruct);
    match expand(item) {
        Ok(ts) => ts.into(),
        Err(err) => err.to_compile_error().into(),
    }
}

/// Implements `alloy_core::ComponentMeta`:
/// `#[alloy_core::component(name = "loki.source.file", stability = Experimental)]`.
/// Stability is one of `Experimental`, `PublicPreview` or `GenerallyAvailable`.
#[proc_macro_attribute]
pub fn component(attr: TokenStream, item: TokenStream) -> TokenStream {
    let item = parse_macro_input!(item as syn::DeriveInput);

    let mut name = None;
    let mut stability = None;
    let parser = syn::meta::parser(|meta| {
        if meta.path.is_ident("name") {
            name = Some(meta.value()?.parse::<syn::LitStr>()?);
            Ok(())
        } else if meta.path.is_ident("stability") {
            let ident: syn::Ident = meta.value()?.parse()?;
            match ident.to_string().as_str() {
                "Experimental" | "PublicPreview" | "GenerallyAvailable" => {
                    stability = Some(ident);
                    Ok(())
                }
                _ => Err(syn::Error::new_spanned(
                    ident,
                    "expected Experimental, PublicPreview or GenerallyAvailable",
                )),
            }
        } else {
            Err(meta.error("expected name or stability"))
        }
    });
    parse_macro_input!(attr with parser);

    let (Some(name), Some(stability)) = (name, stability) else {
        return syn::Error::new(Span::call_site(), "component needs name and stability")
            .to_compile_error()
            .into();
    };

    let ident = &item.ident;
    let (impl_generics, ty_generics, where_clause) = item.generics.split_for_impl();
    quote! {
        #item

        impl #impl_generics ::alloy_core::ComponentMeta for #ident #ty_generics #where_clause {
            const NAME: &'static str = #name;
            const STABILITY: ::alloy_core::schema::Stability =
                ::alloy_core::schema::Stability::#stability;
        }
    }
    .into()
}

#[derive(Clone, Copy)]
enum Ty {
    Uint(&'static str, u32),
    Int(&'static str, u32),
    Float(&'static str),
    Bool,
    Duration,
    String,
    StringMap,
}

struct FieldSpec {
    ident: syn::Ident,
    ty: Ty,
    doc: String,
    default: Option<TokenStream2>,
    min: Option<TokenStream2>,
    max: Option<TokenStream2>,
}

fn expand(mut item: ItemStruct) -> syn::Result<TokenStream2> {
    if !item.generics.params.is_empty() {
        return Err(syn::Error::new_spanned(&item.generics, "arguments can't be generic"));
    }
    let Fields::Named(fields) = &mut item.fields else {
        return Err(syn::Error::new_spanned(&item.ident, "arguments must have named fields"));
    };

    let mut specs = Vec::new();
    for field in fields.named.iter_mut() {
        let spec = parse_field(field)?;
        field.attrs.retain(|a| !a.path().is_ident("arg"));
        if let Ty::Duration = spec.ty {
            field.attrs.push(syn::parse_quote! {
                #[serde(deserialize_with = "::alloy_core::__private::duration_nanos")]
            });
        }
        specs.push(spec);
    }

    let ident = &item.ident;

    let schema = specs.iter().map(|s| {
        let name = s.ident.to_string();
        let ty = field_type(s.ty);
        let doc = &s.doc;
        let opt = |v: &Option<TokenStream2>| match v {
            Some(v) => quote! { ::core::option::Option::Some(#v) },
            None => quote! { ::core::option::Option::None },
        };
        let (default, min, max) = (opt(&s.default), opt(&s.min), opt(&s.max));
        quote! {
            ::alloy_core::schema::Field {
                name: #name,
                ty: #ty,
                default: #default,
                min: #min,
                max: #max,
                doc: #doc,
            }
        }
    });

    Ok(quote! {
        #[derive(::alloy_core::serde::Deserialize)]
        #[serde(crate = "::alloy_core::serde", deny_unknown_fields)]
        #item

        impl ::alloy_core::Arguments for #ident {
            fn schema() -> &'static [::alloy_core::schema::Field] {
                const SCHEMA: &[::alloy_core::schema::Field] = &[#(#schema,)*];
                SCHEMA
            }
        }
    })
}

fn parse_field(field: &syn::Field) -> syn::Result<FieldSpec> {
    let ty = parse_type(&field.ty)?;

    let doc = field
        .attrs
        .iter()
        .filter(|a| a.path().is_ident("doc"))
        .filter_map(|a| match &a.meta {
            syn::Meta::NameValue(nv) => match &nv.value {
                Expr::Lit(ExprLit { lit: Lit::Str(s), .. }) => Some(s.value().trim().to_string()),
                _ => None,
            },
            _ => None,
        })
        .collect::<Vec<_>>()
        .join(" ");

    let mut spec = FieldSpec {
        ident: field.ident.clone().unwrap(),
        ty,
        doc,
        default: None,
        min: None,
        max: None,
    };

    for attr in field.attrs.iter().filter(|a| a.path().is_ident("arg")) {
        attr.parse_nested_meta(|meta| {
            let bounds = !matches!(ty, Ty::Bool | Ty::String | Ty::StringMap);
            let slot = if meta.path.is_ident("default") {
                &mut spec.default
            } else if meta.path.is_ident("min") && bounds {
                &mut spec.min
            } else if meta.path.is_ident("max") && bounds {
                &mut spec.max
            } else if bounds {
                return Err(meta.error("expected default, min or max"));
            } else {
                return Err(meta.error("expected default, only numbers and durations have min or max"));
            };
            let expr: Expr = meta.value()?.parse()?;
            *slot = Some(value(ty, &expr)?);
            Ok(())
        })?;
    }

    Ok(spec)
}

fn parse_type(ty: &Type) -> syn::Result<Ty> {
    let unsupported = || {
        syn::Error::new_spanned(
            ty,
            "unsupported argument type, expected an integer, float, bool, Duration, String or a map of String to String",
        )
    };
    let Type::Path(path) = ty else {
        return Err(unsupported());
    };
    let Some(last) = path.path.segments.last() else {
        return Err(unsupported());
    };
    Ok(match last.ident.to_string().as_str() {
        "u8" => Ty::Uint("u8", 8),
        "u16" => Ty::Uint("u16", 16),
        "u32" => Ty::Uint("u32", 32),
        "u64" => Ty::Uint("u64", 64),
        "i8" => Ty::Int("i8", 8),
        "i16" => Ty::Int("i16", 16),
        "i32" => Ty::Int("i32", 32),
        "i64" => Ty::Int("i64", 64),
        "f32" => Ty::Float("f32"),
        "f64" => Ty::Float("f64"),
        "bool" => Ty::Bool,
        "Duration" => Ty::Duration,
        "String" => Ty::String,
        "BTreeMap" | "HashMap" => {
            let syn::PathArguments::AngleBracketed(args) = &last.arguments else {
                return Err(unsupported());
            };
            let is_string = |arg: &syn::GenericArgument| {
                matches!(arg, syn::GenericArgument::Type(Type::Path(p))
                    if p.path.segments.last().is_some_and(|s| s.ident == "String"))
            };
            if args.args.len() != 2 || !args.args.iter().all(is_string) {
                return Err(unsupported());
            }
            Ty::StringMap
        }
        _ => return Err(unsupported()),
    })
}

fn field_type(ty: Ty) -> TokenStream2 {
    let variant = match ty {
        Ty::Uint(name, _) | Ty::Int(name, _) | Ty::Float(name) => name.to_uppercase(),
        Ty::Bool => "Bool".to_string(),
        Ty::Duration => "Duration".to_string(),
        Ty::String => "String".to_string(),
        Ty::StringMap => "StringMap".to_string(),
    };
    let ident = syn::Ident::new(&variant, Span::call_site());
    quote! { ::alloy_core::schema::FieldType::#ident }
}

/// Turns an `#[arg]` value into a `schema::Value` expression, checking it
/// fits the field's type.
fn value(ty: Ty, expr: &Expr) -> syn::Result<TokenStream2> {
    let (negative, lit) = match expr {
        Expr::Lit(ExprLit { lit, .. }) => (false, lit),
        Expr::Unary(ExprUnary { op: UnOp::Neg(_), expr, .. }) => match &**expr {
            Expr::Lit(ExprLit { lit, .. }) => (true, lit),
            _ => return Err(syn::Error::new_spanned(expr, "expected a literal")),
        },
        _ => return Err(syn::Error::new_spanned(expr, "expected a literal")),
    };
    let err = |msg: &str| Err(syn::Error::new_spanned(expr, msg));

    match (ty, lit) {
        (Ty::Uint(name, bits), Lit::Int(i)) if !negative => {
            let n: u64 = i.base10_parse()?;
            if bits < 64 && n >> bits != 0 {
                return err(&format!("doesn't fit in {name}"));
            }
            Ok(quote! { ::alloy_core::schema::Value::Uint(#n) })
        }
        (Ty::Int(name, bits), Lit::Int(i)) => {
            let n: i64 = i.base10_parse()?;
            let n = if negative { -n } else { n };
            let lo = -(1i128 << (bits - 1));
            let hi = (1i128 << (bits - 1)) - 1;
            if (n as i128) < lo || (n as i128) > hi {
                return err(&format!("doesn't fit in {name}"));
            }
            Ok(quote! { ::alloy_core::schema::Value::Int(#n) })
        }
        (Ty::Float(_), Lit::Float(f)) => {
            let v: f64 = f.base10_parse()?;
            let v = if negative { -v } else { v };
            Ok(quote! { ::alloy_core::schema::Value::Float(#v) })
        }
        (Ty::Float(_), Lit::Int(i)) => {
            let v: f64 = i.base10_parse()?;
            let v = if negative { -v } else { v };
            Ok(quote! { ::alloy_core::schema::Value::Float(#v) })
        }
        (Ty::Bool, Lit::Bool(b)) if !negative => {
            let v = b.value;
            Ok(quote! { ::alloy_core::schema::Value::Bool(#v) })
        }
        (Ty::Duration, Lit::Str(s)) if !negative => match parse_duration(&s.value()) {
            Some(ns) => Ok(quote! {
                ::alloy_core::schema::Value::Duration(::std::time::Duration::from_nanos(#ns))
            }),
            None => err("expected a duration like \"1s\" or \"500ms\""),
        },
        (Ty::String, Lit::Str(s)) if !negative => {
            let v = s.value();
            Ok(quote! { ::alloy_core::schema::Value::Str(#v) })
        }
        (Ty::StringMap, _) => err("map fields can't have a default, min or max"),
        _ => err("value doesn't match the field's type"),
    }
}

/// Parses durations like "1h30m" or "500ms" into nanoseconds.
fn parse_duration(s: &str) -> Option<u64> {
    const UNITS: [(&str, u64); 7] = [
        ("ns", 1),
        ("us", 1_000),
        ("ms", 1_000_000),
        ("s", 1_000_000_000),
        ("m", 60_000_000_000),
        ("h", 3_600_000_000_000),
        ("d", 86_400_000_000_000),
    ];

    let mut total: u64 = 0;
    let mut rest = s;
    if rest.is_empty() {
        return None;
    }
    while !rest.is_empty() {
        let digits = rest.find(|c: char| !c.is_ascii_digit())?;
        if digits == 0 {
            return None;
        }
        let n: u64 = rest[..digits].parse().ok()?;
        rest = &rest[digits..];
        let unit_len = rest.find(|c: char| c.is_ascii_digit()).unwrap_or(rest.len());
        let unit = &rest[..unit_len];
        let (_, scale) = UNITS.iter().find(|(name, _)| *name == unit)?;
        total = total.checked_add(n.checked_mul(*scale)?)?;
        rest = &rest[unit_len..];
    }
    Some(total)
}
