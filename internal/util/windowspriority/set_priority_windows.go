//go:build windows

package windowspriority

import (
	"fmt"
	"iter"
	"maps"
	"strings"

	"golang.org/x/sys/windows"
)

// This is the default value in the alloycli package. We don't need constants
// for the other values because they are not used in code.
const PriorityNormal = "normal"

var priorityMap = map[string]uint32{
	"above_normal": windows.ABOVE_NORMAL_PRIORITY_CLASS,
	"below_normal": windows.BELOW_NORMAL_PRIORITY_CLASS,
	"high":         windows.HIGH_PRIORITY_CLASS,
	"idle":         windows.IDLE_PRIORITY_CLASS,
	PriorityNormal: windows.NORMAL_PRIORITY_CLASS,
	"realtime":     windows.REALTIME_PRIORITY_CLASS,
}

func PriorityValues() iter.Seq[string] {
	return maps.Keys(priorityMap)
}

func TranslatePriority(priorityString string) (uint32, error) {
	priority, ok := priorityMap[strings.ToLower(priorityString)]
	if !ok {
		return 0, fmt.Errorf("invalid priority: %s", priorityString)
	}
	return priority, nil
}

func SetPriority(priorityString string) error {
	priority, err := TranslatePriority(priorityString)
	if err != nil {
		return err
	}

	if err := windows.SetPriorityClass(windows.CurrentProcess(), priority); err != nil {
		return fmt.Errorf("failed to set priority: %w", err)
	}

	return nil
}
