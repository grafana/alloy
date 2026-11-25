// Package specs lists the components otelgen generates.
package specs

import (
	"slices"

	"github.com/grafana/alloy/internal/component/otelcol/otelgen/specs/connectors"
	"github.com/grafana/alloy/internal/component/otelcol/otelgen/specs/processors"
)

var All = slices.Concat(processors.All, connectors.All)
