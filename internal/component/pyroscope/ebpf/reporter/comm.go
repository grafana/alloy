package reporter

// CommMode controls how the process comm is included in profiles.
// Valid values: "label", "stackframe", "both", "none", "".
// "" and "none" are treated the same (comm is not included).
type CommMode string

const (
	CommModeNone       CommMode = "none"
	CommModeLabel      CommMode = "label"
	CommModeStackframe CommMode = "stackframe"
	CommModeBoth       CommMode = "both"
)
