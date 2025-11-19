// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package maccess provides functionality to check if a certain bug in
// copy_from_user_nofault is patched.
//
// There were issues with the Linux kernel function copy_from_user_nofault that
// caused systems to freeze. These issues were fixed with the following patch:
// https://git.kernel.org/pub/scm/linux/kernel/git/torvalds/linux.git/commit/?id=d319f344561de23e810515d109c7278919bff7b0
//
//nolint:lll
package maccess // import "go.opentelemetry.io/ebpf-profiler/maccess"
