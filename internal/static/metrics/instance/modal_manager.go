package instance

import (
	"fmt"
)

// Mode controls how instances are created.
type Mode string

// Types of instance modes
var (
	ModeDistinct Mode = "distinct"
	ModeShared   Mode = "shared"

	DefaultMode = ModeShared
)

// UnmarshalYAML unmarshals a string to a Mode. Fails if the string is
// unrecognized.
func (m *Mode) UnmarshalYAML(unmarshal func(any) error) error {
	*m = DefaultMode

	var plain string
	if err := unmarshal(&plain); err != nil {
		return err
	}

	switch plain {
	case string(ModeDistinct):
		*m = ModeDistinct
		return nil
	case string(ModeShared):
		*m = ModeShared
		return nil
	default:
		return fmt.Errorf("unsupported instance_mode '%s'. supported values 'shared', 'distinct'", plain)
	}
}
