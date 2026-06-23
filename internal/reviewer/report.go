package reviewer

import (
	"fmt"
	"io"
)

func Report(w io.Writer, result *Result) {
	_, _ = fmt.Fprintf(w, "stability required: %s\n", result.StabilityRequired)

	for name, comp := range result.Components {
		_, _ = fmt.Fprintf(w, "%s\n", name)
		_, _ = fmt.Fprint(w, "supported platforms: \n")
		for _, p := range comp.Metadata.Platforms {
			_, _ = fmt.Fprintf(w, "  - %s\n", p)
		}
		_, _ = fmt.Fprint(w, "requirements: \n")
		for _, r := range comp.Metadata.Requirements {
			_, _ = fmt.Fprintf(w, "  - %s\n", r.Description)
		}
	}
}
