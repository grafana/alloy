package printer

import (
	"io"
	"text/tabwriter"
)

// A trimmer is an io.Writer which filters tabwriter.Escape characters,
// trailing blanks and tabs from lines, and converting \f and \v characters
// into \n and \t (if no text/tabwriter is used when printing).
//
// Text wrapped by tabwriter.Escape characters is written to the underlying
// io.Writer unmodified.
type trimmer struct {
	next  io.Writer
	state int
	space []byte
}

const (
	trimStateSpace  = iota // Trimmer is reading space characters
	trimStateEscape        // Trimmer is reading escaped characters
	trimStateText          // Trimmer is reading text
)

func (t *trimmer) discardWhitespace() {
	t.state = trimStateSpace
	t.space = t.space[0:0]
}

func (t *trimmer) Write(data []byte) (n int, err error) {
	// textStart holds the index of the start of a chunk of text not containing
	// whitespace. It is reset every time a new chunk of text is encountered.
	var textStart int

	for off, b := range data {
		// Convert \v to \t
		if b == '\v' {
			b = '\t'
		}

		switch t.state {
		case trimStateSpace:
			// Accumulate tabs and spaces in t.space until finding a non-tab or
			// non-space character.
			//
			// If we find a newline, we write it directly and discard our pending
			// whitespace (so that trailing whitespace up to the newline is ignored).
			//
			// If we find a tabwriter.Escape or text character we transition states.
			switch b {
			case '\t', ' ':
				t.space = append(t.space, b)
			case '\n', '\f':
				// Discard all unwritten whitespace before the end of the line and write
				// a newline.
				t.discardWhitespace()
				_, err = t.next.Write([]byte("\n"))
			case tabwriter.Escape:
				_, err = t.next.Write(t.space)
				t.state = trimStateEscape
				textStart = off + 1 // Skip escape character
			default:
				// Non-space character. Write our pending whitespace
				// and then move to text state.
				_, err = t.next.Write(t.space)
				t.state = trimStateText
				textStart = off
			}

		case trimStateText:
			// We're reading a chunk of text. Accumulate characters in the chunk
			// until we find a whitespace character or a tabwriter.Escape.
			switch b {
			case '\t', ' ':
				_, err = t.next.Write(data[textStart:off])
				t.discardWhitespace()
				t.space = append(t.space, b)
			case '\n', '\f':
				_, err = t.next.Write(data[textStart:off])
				t.discardWhitespace()
				if err == nil {
					_, err = t.next.Write([]byte("\n"))
				}
			case tabwriter.Escape:
				_, err = t.next.Write(data[textStart:off])
				t.state = trimStateEscape
				textStart = off + 1 // +1: skip tabwriter.Escape
			}

		case trimStateEscape:
			// Accumulate everything until finding the closing tabwriter.Escape.
			if b == tabwriter.Escape {
				_, err = t.next.Write(data[textStart:off])
				t.discardWhitespace()
			}

		default:
			panic("unreachable")
		}
		if err != nil {
			return off, err
		}
	}
	n = len(data)

	// Flush the remainder of the text (as long as it's not whitespace).
	switch t.state {
	case trimStateEscape, trimStateText:
		_, err = t.next.Write(data[textStart:n])
		t.discardWhitespace()
	}

	return
}
