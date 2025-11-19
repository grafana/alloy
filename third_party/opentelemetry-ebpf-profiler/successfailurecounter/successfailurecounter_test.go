// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package successfailurecounter

import (
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
)

func defaultToSuccess(t *testing.T, sfc SuccessFailureCounter, n int) {
	t.Helper()
	defer sfc.DefaultToSuccess()

	if n%2 == 0 {
		sfc.ReportSuccess()
	} else if n%3 == 0 {
		sfc.ReportFailure()
	}
}

func defaultToFailure(t *testing.T, sfc SuccessFailureCounter, n int) {
	t.Helper()
	defer sfc.DefaultToFailure()

	if n%2 == 0 {
		sfc.ReportSuccess()
	} else if n%3 == 0 {
		sfc.ReportFailure()
	}
}

func TestSuccessFailureCounter(t *testing.T) {
	tests := map[string]struct {
		call            func(*testing.T, SuccessFailureCounter, int)
		input           int
		expectedSucess  uint64
		expectedFailure uint64
	}{
		"default success - no report": {
			call:           defaultToSuccess,
			input:          1,
			expectedSucess: 1,
		},
		"default success - report success": {
			call:           defaultToSuccess,
			input:          2,
			expectedSucess: 1,
		},
		"default success - report failure": {
			call:            defaultToSuccess,
			input:           3,
			expectedFailure: 1,
		},
		"default failure - no report": {
			call:            defaultToFailure,
			input:           1,
			expectedFailure: 1,
		},
		"default failure - report success": {
			call:           defaultToFailure,
			input:          2,
			expectedSucess: 1,
		},
		"default failure - report failure": {
			call:            defaultToFailure,
			input:           3,
			expectedFailure: 1,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			var success, failure atomic.Uint64
			sfc := New(&success, &failure)
			test.call(t, sfc, test.input)
			assert.Equal(t, test.expectedSucess, success.Load())
			assert.Equal(t, test.expectedFailure, failure.Load())
		})
	}
}
