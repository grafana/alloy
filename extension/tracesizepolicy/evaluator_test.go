// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tracesizepolicy

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/pkg/samplingpolicy"
)

func TestParseConfig(t *testing.T) {
	tests := []struct {
		name      string
		raw       map[string]any
		wantBytes uint64
		wantErr   string
	}{
		{name: "int", raw: map[string]any{"min_bytes": 5_000_000}, wantBytes: 5_000_000},
		{name: "int64", raw: map[string]any{"min_bytes": int64(5_000_000)}, wantBytes: 5_000_000},
		{name: "float64", raw: map[string]any{"min_bytes": float64(5_000_000)}, wantBytes: 5_000_000},
		{name: "missing", raw: map[string]any{}, wantErr: "missing required min_bytes"},
		{name: "zero", raw: map[string]any{"min_bytes": 0}, wantErr: "greater than 0"},
		{name: "wrong type", raw: map[string]any{"min_bytes": "big"}, wantErr: "must be a number"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := parseConfig(tt.raw)
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantBytes, cfg.minBytes)
		})
	}
}

func TestSizeEvaluator_Evaluate(t *testing.T) {
	e := &sizeEvaluator{minBytes: 5_000_000}

	small := &samplingpolicy.TraceData{SizeBytes: 4_999_999}
	decision, err := e.Evaluate(context.Background(), pcommon.NewTraceIDEmpty(), small)
	require.NoError(t, err)
	assert.Equal(t, samplingpolicy.NotSampled, decision)

	big := &samplingpolicy.TraceData{SizeBytes: 5_000_000}
	decision, err = e.Evaluate(context.Background(), pcommon.NewTraceIDEmpty(), big)
	require.NoError(t, err)
	assert.Equal(t, samplingpolicy.Sampled, decision)

	assert.False(t, e.IsStateful())
}
