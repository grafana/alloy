// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//! Cross-language symbol demangling.

use smallvec::SmallVec;
use std::borrow::Cow;

/// Strips disambiguation suffixes commonly appended to function clones.
///
/// Modern compilers frequently create specialized versions of functions that
/// factor in additional information from a call site, e.g. arguments that are
/// constants. The corresponding optimization passes append dot-prefixed suffixes
/// like `.isra.0` to the function name to disambiguate them from the regular
/// function instance.
///
/// binutils' demangler[1] will simply consider anything after a `.` to be a
/// clone suffix: this works because they only demangle Rust and C++ which
/// both won't otherwise ever have single dots within their name (only double-
/// dots). However, we also care about Go which DOES have regular dots but
/// otherwise doesn't have any prefix that allows clear separation from C
/// symbols with a clone suffix (e.g. `runtime.saveg`). We thus need to keep
/// a white-list of specific known clone suffixes instead, which might turn
/// out to be a bit of a maintenance burden.
///
/// [1]: https://github.com/bminor/binutils-gdb/blob/978042640c/libiberty/cp-demangle.c#L4043
///
/// The following shell command can be used to get a list of possible suffixes:
///
/// ```bash
/// llvm-readelf --syms ~devel/libxul.so.dbg | awk '{ print $8 }' |     \
//      rg --passthru -F '..' -r 'DOTDOT' | rg -F . |                   \
//      cut -d . -f 2- | tr '.' '\n' | sort | uniq -c | sort -nr
/// ```
fn strip_clone_suffixes(mut name: &str) -> &str {
    // Strip suffixes like ".llvm.9420829416740162726", ".constprop.0", etc.
    for suffix in &[".clone.", ".constprop.", ".llvm.", ".isra.", ".part."] {
        if let Some(pos) = name.rfind(suffix) {
            if name[pos + suffix.len()..]
                .chars()
                .take_while(|&x| x != '.')
                .all(|x| x.is_ascii_digit())
            {
                name = &name[..pos];
            }
        }
    }

    // Strip ".cold" suffix.
    if let Some(stripped) = name.strip_suffix(".cold") {
        name = stripped;
    }

    name
}

fn could_be_rust_symbol(name: &str) -> bool {
    // V0 mangling.
    if name.starts_with("_R") {
        return true;
    }

    // Legacy mangling: _ZN.*17h[a-zA-Z0-9]{16}E
    if name.starts_with("_ZN")
        && name.ends_with('E')
        && name.len() > 3 + 3 + 16 + 1
        && &name[name.len() - 3 - 16 - 1..][..3] == "17h"
        && name[name.len() - 16 - 1..][..16]
            .chars()
            .all(|x| x.is_ascii_hexdigit())
    {
        return true;
    }

    false
}

fn could_be_itanium_abi_cxx_symbol(name: &str) -> bool {
    // With the exception of MSVC, this is the C++ mangling format emitted
    // by essentially all modern C++ compilers.
    //
    // https://clang.llvm.org/doxygen/ItaniumMangle_8cpp_source.html
    // https://itanium-cxx-abi.github.io/cxx-abi/abi.html#mangling
    name.starts_with("_Z") || name.starts_with("___Z")
}

/// C++ name formatter dropping template arguments.
#[derive(Debug, Default)]
struct CxxFormatter {
    buf: String,
    stack: SmallVec<[cpp_demangle::DemangleNodeType; 32]>,
}

impl CxxFormatter {
    pub fn finalize(self) -> String {
        debug_assert!(self.stack.is_empty());
        self.buf
    }
}

impl cpp_demangle::DemangleWrite for CxxFormatter {
    fn push_demangle_node(&mut self, ty: cpp_demangle::DemangleNodeType) {
        self.stack.push(ty);
    }

    fn write_string(&mut self, s: &str) -> std::fmt::Result {
        if matches!(s, "<" | ">") {
            return Ok(());
        }

        use cpp_demangle::DemangleNodeType::TemplateArgs;
        if self.stack.iter().any(|&x| x == TemplateArgs) {
            return Ok(());
        }

        self.buf.push_str(s);

        Ok(())
    }

    fn pop_demangle_node(&mut self) {
        self.stack.pop();
    }
}

/// Demangles the given symbol name.
pub fn demangle(mut name: &str) -> Cow<'_, str> {
    name = strip_clone_suffixes(name);

    if could_be_rust_symbol(name) {
        if let Ok(demangler) = rustc_demangle::try_demangle(name) {
            // The alternate formatting using `#` suppresses the hash suffix.
            return Cow::Owned(format!("{:#}", demangler));
        };
    }

    if could_be_itanium_abi_cxx_symbol(name) {
        if let Ok(sym) = cpp_demangle::BorrowedSymbol::new(name.as_bytes()) {
            let mut formatter = CxxFormatter::default();
            let options = cpp_demangle::DemangleOptions::default();
            if let Ok(()) = sym.structured_demangle(&mut formatter, &options) {
                return Cow::Owned(formatter.finalize());
            }
        }
    }

    Cow::Borrowed(name)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn c() {
        assert_eq!(demangle("blah.cold"), "blah");
        assert_eq!(demangle("blah.constprop.0.cold"), "blah");
        assert_eq!(demangle("blah"), "blah");
        assert_eq!(
            demangle("_RustIsNotTheOnlyLangWhoseSymbolsCanStartWith_R"),
            "_RustIsNotTheOnlyLangWhoseSymbolsCanStartWith_R",
        );
    }

    #[test]
    fn cxx() {
        let mangled = concat!(
            "_ZNSt3__111__introsortINS_12__debug_lessINS_6__lessIN14arrow_vendored4date9",
            "time_zoneES5_EEEEPS5_EEvT0_S9_T_NS_15iterator_traitsIS9_E15difference_typeE",
        );
        let demangled = concat!(
            "void std::__1::__introsort(arrow_vendored::date::time_zone*, ",
            "arrow_vendored::date::time_zone*, std::__1::__debug_less, ",
            "std::__1::iterator_traits::difference_type)",
        );
        assert_eq!(demangle(mangled), demangled);

        let mangled = concat!(
            "_ZN7mozilla3dom17Selection_BindingL8get_typeEP9JSContext",
            "N2JS6HandleIP8JSObjectEEPv19JSJitGetterCallArgs.cold",
        );
        let demangled = concat!(
            "mozilla::dom::Selection_Binding::get_type(",
            "JSContext*, JS::Handle, void*, JSJitGetterCallArgs)",
        );
        assert_eq!(demangle(mangled), demangled);

        let mangled = concat!(
            "_ZN5media13MojoDecryptor21DecryptAndDecodeVideoE13scoped_refptrINS_13Decoder",
            "BufferEERKN4base17RepeatingCallbackIFvNS_9Decryptor6StatusES1_INS_10VideoFrameEEEEE",
        );
        let demangled = concat!(
            "media::MojoDecryptor::DecryptAndDecodeVideo(",
            "scoped_refptr, base::RepeatingCallback const&)",
        );
        assert_eq!(demangle(mangled), demangled,);

        let mangled = concat!(
            "_ZN2js8HeapSlot4postEPNS_12NativeObjectENS0_",
            "4KindEjRKN2JS5ValueE.isra.0.cold",
        );
        let demangled = concat!(
            "js::HeapSlot::post(js::NativeObject*, js::HeapSlot::Kind, ",
            "unsigned int, JS::Value const&)"
        );
        assert_eq!(demangle(mangled), demangled);
    }

    #[test]
    fn rust() {
        let mangled = concat!(
            "_ZN50_$LT$$RF$mut$u20$W$u20$as$u20$core..fmt..Write",
            "$GT$10write_char17h40d2a72f9527ade5E.llvm.5999636307758439825",
        );
        assert_eq!(
            demangle(mangled),
            "<&mut W as core::fmt::Write>::write_char",
        );

        let mangled = concat!(
            "_ZN71_$LT$rustc_demangle..legacy..Demangle$u20",
            "$as$u20$core..fmt..Display$GT$3fmt17h48ee277748f854a8E",
        );
        assert_eq!(
            demangle(mangled),
            "<rustc_demangle::legacy::Demangle as core::fmt::Display>::fmt",
        );
    }

    #[test]
    fn go() {
        let names = &[
            "github.com/googleapis/gnostic/openapiv2..stmp_173",
            "runtime.(*mheap).grow",
            "runtime.cmpstring",
            "type..eq.k8s.io/api/core/v1.NodeSystemInfo",
            "go.opemtelemetry.io/ebpf-profiler/libpf.Max[go.shape.uint32_0]",
            concat!(
                "go.opentelemetry.io/ebpf-profiler/libpf.MapKeysToSlice",
                "[go.shape.uint32_0,go.shape.struct {}_1]"
            ),
            concat!(
                r#"type..eq.struct { APIVersion string "json:\"apiVersion,"#,
                r#"omitempty\""; Kind string "json:\"kind,omitempty\"" }"#,
            ),
            concat!(
                "go.opentelemetry.io/ebpf-profiler/libpf/xsync.(*RWMutex[go.shape.struct ",
                "{ go.opentelemetry.io/ebpf-profiler/processmanager/",
                "execinfomanager.interpreterLoaders []go.opentelemetry.io/ebpf-profiler/",
                "interpreter.Loader; go.opentelemetry.io/ebpf-profiler/",
                "processmanager/execinfomanager.ebpf go.opentelemetry.io/ebpf-profiler/",
                "processmanager/ebpf.EbpfHandler; ",
                "go.opentelemetry.io/ebpf-profiler/processmanager/",
                "execinfomanager.reporter go.opentelemetry.io/ebpf-profiler/",
                "interpreter.ReportFrameMetadataFunc; go.opentelemetry.io/ebpf-profiler/",
                "processmanager/execinfomanager.executables map[go.opentelemetry.io/",
                "ebpf-profiler/host.FileID]*go.opentelemetry.io/ebpf-profiler/",
                "processmanager/execinfomanager.entry; go.opentelemetry.io/ebpf-profiler/",
                "processmanager/execinfomanager.unwindInfoIndex ",
                "map[go.opentelemetry.io/ebpf-profiler/libpf/nativeunwind/stackdeltatypes.UnwindInfo",
                "]uint16; go.opentelemetry.io/ebpf-profiler/processmanager/",
                "execinfomanager.numStackDeltaMapPages uint64 }_0]).WUnlock"
            ),
        ];

        // Make sure that Go symbols are passed through untouched
        for &name in names {
            assert_eq!(demangle(name), name);
        }
    }
}
