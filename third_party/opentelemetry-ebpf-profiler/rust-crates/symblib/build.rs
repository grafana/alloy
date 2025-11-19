// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

static PROTO: &str = "../symb-proto/symbfile.proto";

fn main() -> Result<(), Box<dyn std::error::Error>> {
    println!("cargo:rerun-if-changed={PROTO}");
    Ok(prost_build::compile_protos(
        &[PROTO],
        &["../symb-proto"],
    )?)
}
