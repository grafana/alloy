//go:build linux && arm64 && pyroscope_ebpf && !pyroscope_ebpf_no_link

package ebpf

/*
#cgo LDFLAGS: ${SRCDIR}/../../../../target/aarch64-unknown-linux-musl/release/libsymblib_capi.a
*/
import "C"
