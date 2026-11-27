// Package batch provides an otelcol.processor.batch component.
package batch

import (
	"go.opentelemetry.io/collector/processor/batchprocessor"

	"github.com/grafana/alloy/internal/component/otelcol/bridge"
	"github.com/grafana/alloy/internal/featuregate"
)

func init() {
	bridge.RegisterProcessor(batchprocessor.NewFactory(), bridge.ProcessorOptions[batchprocessor.Config]{
		Stability: featuregate.StabilityGenerallyAvailable,
		// Alloy has always documented smaller batches than upstream.
		Defaults: func(cfg *batchprocessor.Config) {
			cfg.SendBatchSize = 2000
			cfg.SendBatchMaxSize = 3000
		},
	})
}

// Arguments configures the otelcol.processor.batch component.
type Arguments = bridge.ProcessorArguments[batchprocessor.Config]
