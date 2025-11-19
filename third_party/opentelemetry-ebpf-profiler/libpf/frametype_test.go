// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package libpf

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFrameTypeFromString(t *testing.T) {
	// Simple check whether all FrameType values can be converted to string and back.
	for _, ft := range []FrameType{
		unknownFrame, PHPFrame, PythonFrame, NativeFrame, KernelFrame, HotSpotFrame, RubyFrame,
		PerlFrame, V8Frame, DotnetFrame, AbortFrame} {
		t.Run(ft.String(), func(t *testing.T) {
			name := ft.String()
			result := FrameTypeFromString(name)
			require.Equal(t, ft, result)
		})
	}
}
