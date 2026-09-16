// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package tracesizepolicy implements a tailsamplingprocessor policy
// extension (see samplingpolicy.Extension) that samples a trace once its
// exact accumulated encoded size reaches a configured threshold. It exists
// so a local diagnostic collector can pass through only unusually large
// traces (e.g. those a production backend would reject) without
// approximating size via span count.
package tracesizepolicy

import (
	"context"

	"go.opentelemetry.io/collector/component"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/pkg/samplingpolicy"
)

type traceSizePolicyExtension struct{}

func newExtension() *traceSizePolicyExtension {
	return &traceSizePolicyExtension{}
}

func (*traceSizePolicyExtension) Start(context.Context, component.Host) error {
	return nil
}

func (*traceSizePolicyExtension) Shutdown(context.Context) error {
	return nil
}

// NewEvaluator is called by tailsamplingprocessor once per policy that
// references this extension's component ID as its `type`. policyName is
// the policy's configured `name`; cfg is the map under the policy's
// `tracesizepolicy:` key.
func (*traceSizePolicyExtension) NewEvaluator(_ string, cfg map[string]any) (samplingpolicy.Evaluator, error) {
	parsed, err := parseConfig(cfg)
	if err != nil {
		return nil, err
	}
	return &sizeEvaluator{minBytes: parsed.minBytes}, nil
}

var (
	_ component.Component      = (*traceSizePolicyExtension)(nil)
	_ samplingpolicy.Extension = (*traceSizePolicyExtension)(nil)
)
