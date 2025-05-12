package foreach

type Arguments struct {
	Collection []any  `alloy:"collection,attr"`
	Var        string `alloy:"var,attr"`
	Id         string `alloy:"id,attr,optional"`

	// EnableMetrics should be false by default.
	// That way users are protected from an explosion of debug metrics
	// if there are many items inside "collection".
	EnableMetrics bool `alloy:"enable_metrics,attr,optional"`
}
