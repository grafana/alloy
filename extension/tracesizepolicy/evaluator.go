// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tracesizepolicy

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/pkg/samplingpolicy"
)

type evaluatorConfig struct {
	minBytes uint64
}

// parseConfig reads the policy-scoped config block (the map under the
// policy's `tracesizepolicy:` key) that tailsamplingprocessor hands to
// NewEvaluator.
func parseConfig(raw map[string]any) (evaluatorConfig, error) {
	var cfg evaluatorConfig

	v, ok := raw["min_bytes"]
	if !ok {
		return cfg, fmt.Errorf("tracesizepolicy: missing required min_bytes")
	}

	switch n := v.(type) {
	case int:
		cfg.minBytes = uint64(n)
	case int64:
		cfg.minBytes = uint64(n)
	case uint64:
		cfg.minBytes = n
	case float64:
		cfg.minBytes = uint64(n)
	default:
		return cfg, fmt.Errorf("tracesizepolicy: min_bytes must be a number, got %T", v)
	}

	if cfg.minBytes == 0 {
		return cfg, fmt.Errorf("tracesizepolicy: min_bytes must be greater than 0")
	}
	return cfg, nil
}

// sizeEvaluator samples a trace once its accumulated encoded size reaches
// minBytes. tailsamplingprocessor already tracks TraceData.SizeBytes
// precisely as spans arrive, so this needs no marshaling of its own.
type sizeEvaluator struct {
	minBytes uint64
}

func (e *sizeEvaluator) Evaluate(_ context.Context, _ pcommon.TraceID, trace *samplingpolicy.TraceData) (samplingpolicy.Decision, error) {
	if trace.SizeBytes >= e.minBytes {
		return samplingpolicy.Sampled, nil
	}
	return samplingpolicy.NotSampled, nil
}

func (e *sizeEvaluator) IsStateful() bool {
	return false
}
