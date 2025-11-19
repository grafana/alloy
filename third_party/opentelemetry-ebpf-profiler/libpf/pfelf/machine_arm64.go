//go:build arm64

// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pfelf // import "go.opentelemetry.io/ebpf-profiler/libpf/pfelf"

import "debug/elf"

const CurrentMachine = elf.EM_AARCH64
