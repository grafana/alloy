// Package processors holds the otelgen specs for otelcol.processor.* components.
package processors

import (
	"github.com/alecthomas/units"
	"github.com/grafana/alloy/internal/component/otelcol/otelgen"
	"github.com/grafana/alloy/internal/featuregate"
	"go.opentelemetry.io/collector/processor/memorylimiterprocessor"
)

var All = []otelgen.Component{
	otelgen.Processor(
		"memorylimiter",
		memorylimiterprocessor.NewFactory,
		otelgen.OverrideStability(
			"already generally available in Alloy, lowering it would be a breaking change",
			featuregate.StabilityGenerallyAvailable,
		),
		otelgen.OverrideArgumentRequired("check_interval"),
		otelgen.OverrideArgumentName("limit_mib", "limit"),
		otelgen.OverrideArgumentGoName("limit_mib", "MemoryLimit"),
		otelgen.OverrideArgumentType[units.Base2Bytes]("limit_mib"),
		otelgen.OverrideArgumentName("spike_limit_mib", "spike_limit"),
		otelgen.OverrideArgumentGoName("spike_limit_mib", "MemorySpikeLimit"),
		otelgen.OverrideArgumentType[units.Base2Bytes]("spike_limit_mib"),
	),
}
