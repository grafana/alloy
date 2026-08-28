// Command stabilitycheck fails CI when a non-GA component is not tracked, or
// its tracking entry is stale, in .stability.yaml.
//
// The component registry is the source of findings. Every experimental or
// public-preview component must have an entry with a reason and a review
// deadline. When the deadline passes, or the code and entry disagree, CI fails
// until a human renews or removes the entry. This mirrors the .govulncheck.yaml
// model, where a justification must not go stale.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"time"

	"github.com/grafana/alloy/internal/component"

	_ "github.com/grafana/alloy/internal/component/all" // Import all component definitions.
)

func main() {
	configPath := flag.String("config", ".stability.yaml", "path to the stability tracking config")
	flag.Parse()

	cfg, err := loadConfig(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load config: %v\n", err)
		os.Exit(2)
	}

	findings := classify(enumerate(), cfg, time.Now())
	report(os.Stdout, findings)
	if len(findings) > 0 {
		os.Exit(1)
	}
}

// enumerate reads every registered component from the runtime registry.
func enumerate() []registeredComponent {
	names := component.AllNames()
	sort.Strings(names)

	out := make([]registeredComponent, 0, len(names))
	for _, name := range names {
		reg, ok := component.Get(name)
		if !ok {
			continue
		}
		// Community components have an undefined stability whose String() is not
		// quoted; Unquote fails and leaves stability empty, which is fine
		// because classify checks the Community flag first.
		stability, _ := strconv.Unquote(reg.Stability.String())
		out = append(out, registeredComponent{
			Name:      reg.Name,
			Stability: stability,
			Community: reg.Community,
		})
	}
	return out
}

// report writes each finding, then a summary. It mirrors the govulncheck
// wrapper report style.
func report(w io.Writer, findings []finding) {
	if len(findings) == 0 {
		_, _ = fmt.Fprintln(w, "stabilitycheck: all non-GA components and features are tracked and current")
		return
	}
	for _, f := range findings {
		_, _ = fmt.Fprintf(w, "  [FAIL] (%s) %s: %s\n", f.section, f.name, f.message)
	}
	_, _ = fmt.Fprintf(w, "  → %d problem(s) found\n", len(findings))
}
