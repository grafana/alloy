//go:build linux && amd64 && pyroscope_ebpf && !pyroscope_ebpf_no_link

package ebpf

/*
#cgo LDFLAGS: ${SRCDIR}/../../../../target/x86_64-unknown-linux-musl/release/libsymblib_capi.a
*/
import "C"
