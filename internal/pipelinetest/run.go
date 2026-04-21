package pipelinetest

import (
	"github.com/grafana/alloy/internal/pipelinetest/harness"
)

type TestConfig struct {
	DataPath string
}

func RunTest(schema TestSchema, cfg TestConfig) error {
	alloy, err := harness.NewAlloy(harness.Config{
		SinkID:   "pipelinetest.sink.out",
		DataPath: cfg.DataPath,
		Source:   withPrelude(schema),
	})
	if err != nil {
		return err
	}
	defer alloy.Stop()

	if err := produceInputs(alloy, schema.Inputs); err != nil {
		return err
	}

	assertions, err := buildAssertions(schema.Assert)
	if err != nil {
		return err
	}

	return alloy.Assert(assertions...)
}
