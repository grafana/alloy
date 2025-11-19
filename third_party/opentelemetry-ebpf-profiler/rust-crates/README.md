Rust components
===============

This directory contains the Rust components for symbolization of native traces.
They are built using the `cargo` build system. Please refer to the README
documents in the subdirectories for details.

## Source code documentation

> [!TIP]
>
> If you're trying to familiarize yourself with the codebase, this is heavily
> recommended. All the important documentation and `README`s are included into
> the rustdoc built documentation, and the generated doc is much more structured
> than what you'd get by just browsing through the repository.

The source code is extensively documented with `rustdoc`, which is invoked
through cargo.

```bash
# Build documentation for our Rust crates and open it in a browser window
cargo doc --document-private-items --workspace --open
```

By default, this will open the documentation for `symblib`.

## Import style

Whenever the name of a type or function that is being imported isn't necessarily
unique, we instead import the module that contains it and then use the module
name to qualify the access. This is essentially similar to how things are done
in Golang.

If the item being important has a very significant, unique name within the code-
base, it's also acceptable to import (`use`) that type directly and refer to it
without additional qualification.

<details>
<summary>Examples</summary>

There are many different modules that expose `File` and `Range` types. Import
the module instead and qualify the items with `module::item`.

```rust
use std::fs;
use symblib::objfile;

let a: fs::File = todo!();
let b: objfile::File = todo!();
```

```rust
use std::ops;
use symblib::symbfile;

let a: ops::Range<u64> = todo!();
let b: symbfile::Range = todo!();
```

`GoRuntimeInfo` is a very unique name that is unlikely to cause confusion even
without further qualification. Import item directly.

```rust
use symblib::gosym::GoRuntimeInfo;

let a: GoRuntimeInfo<'static> = todo!();
```

</details>
