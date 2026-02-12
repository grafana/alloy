//go:build !windows

package testutil

import (
	"strings"

	"github.com/grafana/alloy/internal/component/discovery"
)

// PathEndsWith checks if any target's __path__ ends with the given suffix.
// On Unix, this is case-sensitive.
func PathEndsWith(sources []discovery.Target, suffix string) bool {
	for _, s := range sources {
		p, _ := s.Get("__path__")
		if strings.HasSuffix(p, suffix) {
			return true
		}
	}
	return false
}
