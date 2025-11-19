// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package libpf

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTraceType(t *testing.T) {
	tests := []struct {
		ty     FrameType
		isErr  bool
		interp InterpreterType
		str    string
	}{
		{
			ty:     AbortFrame,
			isErr:  true,
			interp: UnknownInterp,
			str:    "abort-marker",
		},
		{
			ty:     PythonFrame,
			isErr:  false,
			interp: Python,
			str:    "cpython",
		},
		{
			ty:     NativeFrame.Error(),
			isErr:  true,
			interp: Native,
			str:    "native",
		},
	}

	for _, test := range tests {
		assert.Equal(t, test.isErr, test.ty.IsError())
		assert.Equal(t, test.interp, test.ty.Interpreter())
		assert.Equal(t, test.str, test.ty.String())
	}
}
