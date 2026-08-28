// Command listcomponents dumps the list of known Alloy components with their
// stability.
package main

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"text/tabwriter"

	"github.com/grafana/alloy/internal/component"

	_ "github.com/grafana/alloy/internal/component/all" // Import all component definitions
)

func main() {
	components := component.AllNames()
	sort.Strings(components)

	tw := tabwriter.NewWriter(os.Stdout, 0, 8, 0, '\t', 0)
	defer func() { _ = tw.Flush() }()

	for _, name := range components {
		reg, ok := component.Get(name)
		if !ok {
			// Not possible, but we'll ignore it anyway.
			continue
		}

		stability, _ := strconv.Unquote(reg.Stability.String())
		_, _ = fmt.Fprintf(tw, "%s \t%s\n", reg.Name, stability)
	}
}
