// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package hash

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUint64(t *testing.T) {
	tests := map[string]struct {
		input  uint64
		expect uint64
	}{
		"0":          {input: 0, expect: 0},
		"1":          {input: 1, expect: 12994781566227106604},
		"uint16 max": {input: uint64(math.MaxUint16), expect: 6444452806975366496},
		"uint32 max": {input: uint64(math.MaxUint32), expect: 14731816277868330182},
		"uint64 max": {input: math.MaxUint64, expect: 7256831767414464289},
	}

	for name, testcase := range tests {
		t.Run(name, func(t *testing.T) {
			result := Uint64(testcase.input)
			assert.Equal(t, testcase.expect, result)
		})
	}
}

func TestUint32(t *testing.T) {
	tests := map[string]struct {
		input  uint32
		expect uint32
	}{
		"0":          {input: 0, expect: 0},
		"1":          {input: 1, expect: 1364076727},
		"uint16 max": {input: uint32(math.MaxUint16), expect: 2721820263},
		"uint32 max": {input: math.MaxUint32, expect: 2180083513},
	}

	for name, testcase := range tests {
		t.Run(name, func(t *testing.T) {
			result := Uint32(testcase.input)
			assert.Equal(t, testcase.expect, result)
		})
	}
}
