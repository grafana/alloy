package args

import "fmt"

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

func (m CommMode) Validate() error {
	switch m {
	case "", CommModeNone, CommModeLabel, CommModeStackframe, CommModeBoth:
		return nil
	default:
		return fmt.Errorf("invalid comm mode %q, valid values are: %q, %q, %q, %q", m, CommModeNone, CommModeLabel, CommModeStackframe, CommModeBoth)
	}
}

func (m CommMode) Label() bool {
	return m == CommModeLabel || m == CommModeBoth
}

func (m CommMode) Stackframe() bool {
	return m == CommModeStackframe || m == CommModeBoth
}
