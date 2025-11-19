// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package main

// This file contains the magic to include and build the ebpf C-code via CGO.
//
// The approach is to have a TLS variable (struct cgo_ctx *) that describe
// state of the eBPF program. This files defines that, and #includes all the
// eBPF code to be built in this unit. Additionally the main entry point
// "unwind_traces" that setups the TLS, and starts the unwinding is defined here.
// Also the tail call helper "bpf_tail_call" is overridden here, as it works in
// tandem with the main entry point's setjmp.

// todo: enable -Wunused-parameter after 1.25  https://go-review.googlesource.com/c/go/+/642196

/*
#cgo CFLAGS: -Wall -Wextra -Werror
#cgo CFLAGS: -Wno-address-of-packed-member
#cgo CFLAGS: -Wno-unused-label
#cgo CFLAGS: -Wno-sign-compare
#cgo CFLAGS: -Wno-unused-parameter
#cgo CFLAGS: -fno-strict-aliasing
#include "ebpfcode.h"
*/
import "C"
