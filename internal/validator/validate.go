package validator

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"slices"

	"github.com/fatih/color"
	alloy_runtime "github.com/grafana/alloy/internal/runtime"
	"github.com/grafana/alloy/syntax/diag"
)

type state int

const (
	stateOK state = iota
	stateParseError
)

func New(sources map[string][]byte) *Validator {
	sortedNames := make([]string, 0, len(sources))
	states := make(map[string]state, len(sources))
	for name := range sources {
		sortedNames = append(sortedNames, name)
		states[name] = stateOK
	}

	slices.Sort(sortedNames)

	return &Validator{
		source: alloy_runtime.ParseSources(sources),
		names:  sortedNames,
		states: states,
	}
}

type Validator struct {
	source *alloy_runtime.Source
	names  []string
	states map[string]state
}

func (v *Validator) Run() {
	if v.source.HasErrors() {
		// update state for sources that has parse errors
		for _, name := range v.names {
			if err := v.source.Error(name); err != nil {
				v.states[name] = stateParseError
			}
		}
	}
}

func (v *Validator) Report(w io.Writer) {
	bw := bufio.NewWriter(w)

	for _, name := range v.names {
		switch v.states[name] {
		case stateOK:
			if !color.NoColor {
				g := color.New(color.FgGreen, color.Bold)
				_, _ = g.Fprint(bw, "OK: ")
				b := color.New(color.Bold)
				_, _ = b.Fprintf(bw, "%s\n", name)
			} else {
				_, _ = fmt.Fprintf(bw, "OK: %s\n", name)
			}
		case stateParseError:
			printParseErrors(bw, name, v.source.Error(name), v.source.RawConfigs())
		}
	}

	_ = bw.Flush()
}

func printParseErrors(w io.Writer, name string, err error, sources map[string][]byte) {
	var diags diag.Diagnostics
	if errors.As(err, &diags) {
		p := diag.NewPrinter(diag.PrinterConfig{
			Color:              !color.NoColor,
			ContextLinesBefore: 1,
			ContextLinesAfter:  1,
		})
		_ = p.Fprint(w, sources, diags)

		// Print newline after the diagnostics.
		_, _ = w.Write([]byte{'\n'})
		return
	}

	_, _ = fmt.Fprintf(w, "%q: %s\n", name, err)
}
