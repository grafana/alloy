package testutils

import (
	"time"

	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

// DefaultExporterBatch returns the upstream default exporter queue batch config.
// The partition defaults (cache size, idle timeout) are not configurable in
// Alloy, and PartitionConfig is unexported upstream, so they are set field by
// field.
func DefaultExporterBatch() configoptional.Optional[exporterhelper.BatchConfig] {
	batch := exporterhelper.BatchConfig{
		FlushTimeout: 200 * time.Millisecond,
		Sizer:        exporterhelper.RequestSizerTypeItems,
		MinSize:      8192,
	}
	batch.Partition.CacheSize = 10000
	batch.Partition.IdleTimeout = 90 * time.Second
	return configoptional.Default(batch)
}
