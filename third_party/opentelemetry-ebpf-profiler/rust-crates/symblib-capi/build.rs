use std::{env, path::PathBuf, process::Command};

fn main() {
    // Fetch the cargo build manifest.
    let manifest_dir = env::var("CARGO_MANIFEST_DIR").unwrap();
    let output = Command::new("cargo")
        .args(&["metadata", "--format-version=1", "--no-deps"])
        .current_dir(&manifest_dir)
        .output()
        .expect("Failed to execute cargo metadata");

    if !output.status.success() {
        println!("cargo:warning=Failed to get cargo metadata");
        return;
    }

    let metadata: serde_json::Value =
        serde_json::from_slice(&output.stdout).expect("Failed to parse cargo metadata");

    let pkg_name = env::var("CARGO_PKG_NAME").unwrap();
    let packages = metadata["packages"].as_array().unwrap();
    let current_package = packages
        .iter()
        .find(|p| p["name"].as_str().unwrap() == pkg_name)
        .expect("Could not find current package in metadata");

    let targets = current_package["targets"].as_array().unwrap();
    let has_staticlib_target = targets.iter().any(|t| {
        let kinds = t["kind"].as_array().unwrap();
        kinds.iter().any(|k| k.as_str().unwrap() == "staticlib")
    });

    if !has_staticlib_target {
        return;
    }

    let target = match env::var("TARGET") {
        Ok(t) => t,
        Err(_) => return,
    };

    if !target.contains("-linux-musl") {
        return;
    }

    let out_dir = env::var("OUT_DIR").unwrap();

    // Get the target-libdir for the specified target
    // $(shell rustc --target $(RUST_TARGET) --print target-libdir)/self-contained/libunwind.a
    let output = Command::new("rustc")
        .args(&["--target", &target, "--print", "target-libdir"])
        .output()
        .expect("failed to execute rustc");

    if output.status.success() {
        let target_libdir_str = String::from_utf8_lossy(&output.stdout).trim().to_string();
        let libunwind_path = PathBuf::from(target_libdir_str)
            .join("self-contained")
            .join("libunwind.a");

        if libunwind_path.exists() {
            std::fs::copy(libunwind_path, format!("{}/libunwind.a", out_dir)).unwrap();

            println!("cargo:rustc-link-search=native={}", out_dir);
            println!("cargo:rustc-link-lib=static=unwind");
        } else {
            println!("cargo:warning={:?} does not exist", libunwind_path);
        }
    } else {
        println!("cargo:warning=failed to identify target-libdir for libunwind.a");
    }
}
