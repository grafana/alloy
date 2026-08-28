package tail

import "time"

// Line represents a single line read from a tailed file.
type Line struct {
	// Text is the content of the line, with line endings stripped.
	Text string
	// Offset is the byte offset in the file immediately after this line,
	// which is where the next read will occur.
	Offset int64
	// Time is the timestamp when the line was read from the file.
	Time time.Time
}
