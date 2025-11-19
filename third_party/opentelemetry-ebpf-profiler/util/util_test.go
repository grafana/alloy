package util

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNextPowerOfTwo(t *testing.T) {
	tests := []struct {
		name  string
		input uint32
		want  uint32
	}{
		{name: "zero", input: 0, want: 1},
		{name: "one", input: 1, want: 1},
		{name: "two", input: 2, want: 2},
		{name: "three", input: 3, want: 4},
		{name: "four", input: 4, want: 4},
		{name: "five", input: 5, want: 8},
		{name: "six", input: 6, want: 8},
		{name: "0x370", input: 0x370, want: 0x400},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equalf(t, tt.want, NextPowerOfTwo(tt.input),
				"NextPowerOfTwo() = %v, want %v", tt.want, tt.want)
		})
	}
}
