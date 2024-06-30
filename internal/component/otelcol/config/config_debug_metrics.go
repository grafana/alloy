package otelcol

import (
	"fmt"

	"go.opentelemetry.io/collector/config/configtelemetry"
)

type Level string

const (
	// LevelNone indicates that no telemetry data should be collected.
	LevelNone = "none"
	// LevelBasic is the recommended and covers the basics of the service telemetry.
	LevelBasic = "basic"
	// LevelNormal adds some other indicators on top of basic.
	LevelNormal = "normal"
	// LevelDetailed adds dimensions and views to the previous levels.
	LevelDetailed = "detailed"
)

func (l Level) Convert() (configtelemetry.Level, error) {
	switch l {
	case LevelNone:
		return configtelemetry.LevelNone, nil
	case LevelNormal:
		return configtelemetry.LevelNormal, nil
	case LevelBasic:
		return configtelemetry.LevelBasic, nil
	case LevelDetailed:
		return configtelemetry.LevelDetailed, nil
	default:
		return configtelemetry.LevelBasic, fmt.Errorf("unrecognized debug metric level: %s", l)
	}
}

// DebugMetricsArguments configures internal metrics of the components
type DebugMetricsArguments struct {
	DisableHighCardinalityMetrics bool  `alloy:"disable_high_cardinality_metrics,attr,optional"`
	Level                         Level `alloy:"level,attr,optional"`
}

// SetToDefault implements syntax.Defaulter.
func (args *DebugMetricsArguments) SetToDefault() {
	*args = DebugMetricsArguments{
		DisableHighCardinalityMetrics: true,
		Level:                         LevelDetailed,
	}
}
