package pipelinetest

import (
	"context"

	"github.com/grafana/alloy/internal/pipelinetest/harness"
)

type TestConfig struct {
	DataPath string
}

func RunTest(ctx context.Context, schema TestSchema, cfg TestConfig) error {
	if err := validateMocks(schema.Mocks); err != nil {
		return err
	}

	alloy, err := harness.NewAlloy(harness.Config{
		SinkID:   "pipelinetest.sink.out",
		DataPath: cfg.DataPath,
		Source:   withPrelude(schema),
	})
	if err != nil {
		return err
	}
	defer alloy.Stop()

	if err := produceInputs(ctx, alloy, schema.Inputs); err != nil {
		return err
	}

	assertions, err := buildAssertions(schema.Assert)
	if err != nil {
		return err
	}

	return alloy.Assert(assertions...)
}
