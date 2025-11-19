//go:build arm64

// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pacmask // import "go.opentelemetry.io/ebpf-profiler/pacmask"

import (
	"math/rand/v2"
)

// PACIA is an "intrinsic" for the A64 `pacia` instruction.
//
// Given a pointer, a modifier and the secret key stored in a system register
// that isn't visible to EL0 (user-mode), it computes a tag that is inserted
// into the pointer. The bits within the pointer where the tag is stored are
// ignored during address translation. If the hardware doesn't support PAC, the
// `ptr` argument is returned untouched. `ptr` needs to be aligned to 8 bytes.
func PACIA(ptr, modifier uint64) uint64

// GetPACMask determines the mask of where the PAC tag is located in a code
// pointer on ARM64. On architectures != ARM64, this function returns `0`.
//
// The PAC mask varies depending on kernel configs like `CONFIG_ARM64_VA_BITS`
// and `CONFIG_ARM64_MTE`, so we have to determine it dynamically.
func GetPACMask() uint64 {
	// The official [1] way to retrieve the PAC mask on Linux is using ptrace
	// and the `PTRACE_GETREGSET` method. However, since a program cannot debug
	// itself, we'd have to spawn a process, attach for debugging, read the
	// register set and then dispose of that process. Because using `ptrace`
	// without a good reason is probably not exactly something that cloud
	// customers would love us for, this function uses a different approach.
	//
	// The alternative approach generates random 64 bit values with the lower 32
	// bits randomized, asking the CPU to "sign" them with PAC bits. From the
	// signed "pointer", we then remove the 32 bits of randomness from the
	// bottom, leaving us with just the PAC tag bits set by the `pacia`
	// instruction. Repeating this sufficiently often, always ANDing the result
	// with the previous values, after a few iterations, we're statistically
	// pretty much guaranteed to set all bits that belong to the mask.
	//
	// With 32 iterations, assuming 4 PAC bits and an even hash distribution in
	// the PAC bits, the chance for this to work out fine should be:
	//
	//               (1 - 0.5 ** 32) ** 4 = 0.9999999990686774
	//
	// With 64 iterations, IEEE floats are no longer able to express the odds,
	// rounding to `1.0`.
	//
	// [1]: https://www.kernel.org/doc/html/latest/arm64/pointer-authentication.html

	var mask uint64
	for i := 0; i < 64; i++ {
		// The stack pointer on aarch64 needs to be aligned to 8 bytes at all
		// times. The `<< 3` ensures that this is always the case for our fake
		// pointers that will temporarily be placed as a fake stack pointer.
		probe := uint64(rand.Uint32() << 3) //nolint:gosec
		modifier := rand.Uint64()           //nolint:gosec
		probeWithPAC := PACIA(probe, modifier)
		mask |= probeWithPAC & ^uint64(0xFFFF_FFFF)
	}

	return mask
}
