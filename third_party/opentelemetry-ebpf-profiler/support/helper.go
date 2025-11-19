// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// support maps the definitions from headers in the C world into a nice go way
package support // import "go.opentelemetry.io/ebpf-profiler/support"

import "fmt"

// EncodeBiasAndUnwindProgram encodes a bias_and_unwind_program value (for C.PIDPageMappingInfo)
// from a bias and unwind program values.
// This currently assumes a non-negative bias: this encoding may have to be changed if bias can be
// negative.
func EncodeBiasAndUnwindProgram(bias uint64,
	unwindProgram uint8) (uint64, error) {
	if (bias >> 56) > 0 {
		return 0, fmt.Errorf("unsupported bias value (too large): 0x%x", bias)
	}
	return bias | (uint64(unwindProgram) << 56), nil
}

// DecodeBiasAndUnwindProgram decodes the contents of the `bias_and_unwind_program` field in
// C.PIDPageMappingInfo and returns the corresponding bias and unwind program.
func DecodeBiasAndUnwindProgram(biasAndUnwindProgram uint64) (bias uint64, unwindProgram uint8) {
	bias = biasAndUnwindProgram & 0x00FFFFFFFFFFFFFF
	unwindProgram = uint8(biasAndUnwindProgram >> 56)
	return bias, unwindProgram
}
