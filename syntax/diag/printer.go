package diag

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/fatih/color"
	"github.com/grafana/river/token"
)

const tabWidth = 4

// PrinterConfig controls different settings for the Printer.
type PrinterConfig struct {
	// When Color is true, the printer will output with color and special
	// formatting characters (such as underlines).
	//
	// This should be disabled when not printing to a terminal.
	Color bool

	// ContextLinesBefore and ContextLinesAfter controls how many context lines
	// before and after the range of the diagnostic are printed.
	ContextLinesBefore, ContextLinesAfter int
}

// A Printer pretty-prints Diagnostics.
type Printer struct {
	cfg PrinterConfig
}

// NewPrinter creates a new diagnostics Printer with the provided config.
func NewPrinter(cfg PrinterConfig) *Printer {
	return &Printer{cfg: cfg}
}

// Fprint creates a Printer with default settings and prints diagnostics to the
// provided writer. files is used to look up file contents by name for printing
// diagnostics context. files may be set to nil to avoid printing context.
func Fprint(w io.Writer, files map[string][]byte, diags Diagnostics) error {
	p := NewPrinter(PrinterConfig{
		Color:              false,
		ContextLinesBefore: 1,
		ContextLinesAfter:  1,
	})
	return p.Fprint(w, files, diags)
}

// Fprint pretty-prints errors to a writer. files is used to look up file
// contents by name when printing context. files may be nil to avoid printing
// context.
func (p *Printer) Fprint(w io.Writer, files map[string][]byte, diags Diagnostics) error {
	// Create a buffered writer since we'll have many small calls to Write while
	// we print errors.
	//
	// Buffers writers track the first write error received and will return it
	// (if any) when flushing, so we can ignore write errors throughout the code
	// until the very end.
	bw := bufio.NewWriter(w)

	for i, diag := range diags {
		p.printDiagnosticHeader(bw, diag)

		// If there's no ending position, set the ending position to be the same as
		// the start.
		if !diag.EndPos.Valid() {
			diag.EndPos = diag.StartPos
		}

		// We can print the file context if it was found.
		fileContents, foundFile := files[diag.StartPos.Filename]
		if foundFile && diag.StartPos.Filename == diag.EndPos.Filename {
			p.printRange(bw, fileContents, diag)
		}

		// Print a blank line to separate diagnostics.
		if i+1 < len(diags) {
			fmt.Fprintf(bw, "\n")
		}
	}

	return bw.Flush()
}

func (p *Printer) printDiagnosticHeader(w io.Writer, diag Diagnostic) {
	if p.cfg.Color {
		switch diag.Severity {
		case SeverityLevelError:
			cw := color.New(color.FgRed, color.Bold)
			_, _ = cw.Fprintf(w, "Error: ")
		case SeverityLevelWarn:
			cw := color.New(color.FgYellow, color.Bold)
			_, _ = cw.Fprintf(w, "Warning: ")
		}

		cw := color.New(color.Bold)
		_, _ = cw.Fprintf(w, "%s: %s\n", diag.StartPos, diag.Message)
		return
	}

	switch diag.Severity {
	case SeverityLevelError:
		_, _ = fmt.Fprintf(w, "Error: ")
	case SeverityLevelWarn:
		_, _ = fmt.Fprintf(w, "Warning: ")
	}
	fmt.Fprintf(w, "%s: %s\n", diag.StartPos, diag.Message)
}

func (p *Printer) printRange(w io.Writer, file []byte, diag Diagnostic) {
	var (
		start = diag.StartPos
		end   = diag.EndPos
	)

	fmt.Fprintf(w, "\n")

	var (
		lines = strings.Split(string(file), "\n")

		startLine = max(start.Line-p.cfg.ContextLinesBefore, 1)
		endLine   = min(end.Line+p.cfg.ContextLinesAfter, len(lines))

		multiline = end.Line-start.Line > 0
	)

	prefixWidth := len(strconv.Itoa(endLine))

	for lineNum := startLine; lineNum <= endLine; lineNum++ {
		line := lines[lineNum-1]

		// Print line number and margin.
		printPaddedNumber(w, prefixWidth, lineNum)
		fmt.Fprintf(w, " | ")

		if multiline {
			// Use 0 for the column number so we never consider the starting line for
			// showing |.
			if inRange(lineNum, 0, start, end) {
				fmt.Fprint(w, "| ")
			} else {
				fmt.Fprint(w, "  ")
			}
		}

		// Print the line, but filter out any \r and replace tabs with spaces.
		for _, ch := range line {
			if ch == '\r' {
				continue
			}
			if ch == '\t' || ch == '\v' {
				printCh(w, tabWidth, ' ')
				continue
			}
			fmt.Fprintf(w, "%c", ch)
		}

		fmt.Fprintf(w, "\n")

		// Print the focus indicator if we're on a line that needs it.
		//
		// The focus indicator line must preserve whitespace present in the line
		// above it prior to the focus '^' characters. Tab characters are replaced
		// with spaces for consistent printing.
		if lineNum == start.Line || (multiline && lineNum == end.Line) {
			printCh(w, prefixWidth, ' ') // Add empty space where line number would be

			// Print the margin after the blank line number. On multi-line errors,
			// the arrow is printed all the way to the margin, with straight
			// lines going down in between the lines.
			switch {
			case multiline && lineNum == start.Line:
				// |_ would look like an incorrect right angle, so the second bar
				// is dropped.
				fmt.Fprintf(w, " |  _")
			case multiline && lineNum == end.Line:
				fmt.Fprintf(w, " | |_")
			default:
				fmt.Fprintf(w, " | ")
			}

			p.printFocus(w, line, lineNum, diag)
			fmt.Fprintf(w, "\n")
		}
	}
}

// printFocus prints the focus indicator for the line number specified by line.
// The contents of the line should be represented by data so whitespace can be
// retained (injecting spaces where a tab should be, etc.).
func (p *Printer) printFocus(w io.Writer, data string, line int, diag Diagnostic) {
	for i, ch := range data {
		column := i + 1

		if line == diag.EndPos.Line && column > diag.EndPos.Column {
			// Stop printing the formatting line after printing all the ^.
			break
		}

		blank := byte(' ')
		if diag.EndPos.Line-diag.StartPos.Line > 0 {
			blank = byte('_')
		}

		switch {
		case ch == '\t' || ch == '\v':
			printCh(w, tabWidth, blank)
		case inRange(line, column, diag.StartPos, diag.EndPos):
			fmt.Fprintf(w, "%c", '^')
		default:
			// Print a space.
			fmt.Fprintf(w, "%c", blank)
		}
	}
}

func inRange(line, col int, start, end token.Position) bool {
	if line < start.Line || line > end.Line {
		return false
	}

	switch line {
	case start.Line:
		// If the current line is on the starting line, we have to be past the
		// starting column.
		return col >= start.Column
	case end.Line:
		// If the current line is on the ending line, we have to be before the
		// final column.
		return col <= end.Column
	default:
		// Otherwise, every column across all the lines in between
		// is in the range.
		return true
	}
}

func printPaddedNumber(w io.Writer, width int, num int) {
	numStr := strconv.Itoa(num)
	for i := 0; i < width-len(numStr); i++ {
		_, _ = w.Write([]byte{' '})
	}
	_, _ = w.Write([]byte(numStr))
}

func printCh(w io.Writer, count int, ch byte) {
	for i := 0; i < count; i++ {
		_, _ = w.Write([]byte{ch})
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
