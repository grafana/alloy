.PHONY: all clean run-demo

RUST_WORKSPACE_DIR = ../../..
TARGET_DIR = $(RUST_WORKSPACE_DIR)/target/release

all: demo

$(TARGET_DIR)/libsymblib_capi.so: ../src/*.rs
	cargo build --release --manifest-path $(RUST_WORKSPACE_DIR)/Cargo.toml

demo: symblib.h demo.c $(TARGET_DIR)/libsymblib_capi.so
	cc -g -I. -o $@ demo.c -L$(TARGET_DIR) -lsymblib_capi -ldl

run-demo: demo
	LD_LIBRARY_PATH=$(TARGET_DIR) ./demo

clean:
	cargo clean --manifest-path $(RUST_CRATE_DIR)/Cargo.toml
	rm -f demo
