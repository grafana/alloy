package foreach

import "github.com/grafana/alloy/internal/featuregate"

const (
	// BlockName is the block name for foreach blocks.
	BlockName = "foreach"
	// StabilityLevel for foreach blocks.
	StabilityLevel = featuregate.StabilityExperimental
	// TypeTemplate is the block name for template property
	TypeTemplate = "template"
)

type Arguments struct {
	Collection   []any  `alloy:"collection,attr"`
	Var          string `alloy:"var,attr"`
	Id           string `alloy:"id,attr,optional"`
	HashStringId bool   `alloy:"hash_string_id,attr,optional"`

	// EnableMetrics should be false by default.
	// That way users are protected from an explosion of debug metrics
	// if there are many items inside "collection".
	EnableMetrics bool `alloy:"enable_metrics,attr,optional"`
}
