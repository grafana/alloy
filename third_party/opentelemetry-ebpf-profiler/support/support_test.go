package support

import (
	"testing"
	"unsafe"

	"github.com/stretchr/testify/require"
)

func TestSizeOfCGoStruct(t *testing.T) {
	tests := []struct {
		// Name of Go wrapper struct
		name  string
		input uintptr
		want  uintptr
	}{
		{name: "ApmIntProcInfo", input: unsafe.Sizeof(ApmIntProcInfo{}),
			want: sizeof_ApmIntProcInfo},
		{name: "DotnetProcInfo", input: unsafe.Sizeof(DotnetProcInfo{}),
			want: sizeof_DotnetProcInfo},
		{name: "PHPProcInfo", input: unsafe.Sizeof(PHPProcInfo{}),
			want: sizeof_PHPProcInfo},
		{name: "RubyProcInfo", input: unsafe.Sizeof(RubyProcInfo{}),
			want: sizeof_RubyProcInfo},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equalf(t, tt.want, tt.input,
				"unsafe.Sizeof(%v{}) = %v, want %v", tt.name, tt.input, tt.want)
		})
	}
}
