package validator

import (
	"errors"
	"fmt"
	"io"

	"github.com/fatih/color"
	"github.com/grafana/alloy/syntax/diag"
)

func Report(w io.Writer, err error, sources map[string][]byte) {
	var diags diag.Diagnostics
	if errors.As(err, &diags) {
		p := diag.NewPrinter(diag.PrinterConfig{
			Color:              !color.NoColor,
			ContextLinesBefore: 1,
			ContextLinesAfter:  1,
		})
		_ = p.Fprint(w, sources, diags)

		// Print newline after the diagnostics.
		fmt.Println()
		return
	}

	_, _ = fmt.Fprintf(w, "validation failed: %s", err)
}
