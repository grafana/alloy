//go:build !windows
// +build !windows

package windowspriority

import (
	"errors"
	"iter"
)

// This is the default value in the alloycli package
const PriorityNormal = "normal"

func PriorityValues() iter.Seq[string] {
	return nil
}

func TranslatePriority(_ string) (uint32, error) {
	return 0, errors.New("not supported on non-Windows platforms")
}

func SetPriority(_ string) error {
	return nil
}
