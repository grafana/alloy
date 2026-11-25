package alloycli

import (
	"slices"
	"strings"

	"github.com/KimMachineGun/automemlimit/memlimit"
)

// withSystemFallback adds the system memory provider as a fallback to base
// when the AUTOMEMLIMIT_EXPERIMENT value contains "system". automemlimit v1
// no longer reads that variable, so Alloy applies it to keep the documented
// behavior.
func withSystemFallback(base memlimit.Provider, experiments string) memlimit.Provider {
	if slices.Contains(strings.Split(experiments, ","), "system") {
		return memlimit.ApplyFallback(base, memlimit.FromSystem)
	}
	return base
}
