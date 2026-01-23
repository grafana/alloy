package cri

import (
	"strings"
	"unicode"
)

type Flag int8

const (
	// FlagFull indicates a full log line.
	FlagFull Flag = iota
	// FlagPartial indicates a partial log line.
	FlagPartial
)

// String returns the CRI flag representation ("F" or "P").
func (f Flag) String() string {
	switch f {
	case FlagPartial:
		return "P"
	default:
		return ""
	}
}

type Stream int8

const (
	// StreamUnknown indicates the line is not recognized as CRI.
	StreamUnknown Stream = iota
	// StreamStdOut indicates stdout log stream.
	StreamStdOut
	// StreamStdErr indicates stderr log stream.
	StreamStdErr
)

// String returns the CRI stream representation ("stdout" or "stderr").
func (s Stream) String() string {
	switch s {
	case StreamStdOut:
		return "stdout"
	case StreamStdErr:
		return "stderr"
	default:
		return "unknown"
	}
}

type Parsed struct {
	// Timestamp is the raw CRI timestamp field.
	Timestamp string
	// Stream is the CRI stream field (stdout/stderr).
	Stream Stream
	// Flag is the CRI flag field (F/P).
	Flag Flag
	// Content is the log content after the CRI header.
	Content string
}

// ParseCRI parses a CRI formatted log line in a lenient way.
//
// The returned values are safe to retain (Timestamp/Content are owned strings).
// ParseCRI only allocates if the line is valid CRI; for non-CRI lines it returns
// (Parsed{}, false).
func ParseCRI(line string) (Parsed, bool) {
	var (
		timestamp string
		stream    Stream
		flag      Flag
	)

	timestamp, line = parseTimestamp(line)

	stream, line = parseStream(line)
	if stream == StreamUnknown {
		return Parsed{}, false
	}

	flag, line = parseFlag(line)

	return Parsed{
		Timestamp: timestamp,
		Stream:    stream,
		Flag:      flag,
		Content:   line,
	}, true
}

func parseTimestamp(line string) (string, string) {
	i := strings.IndexFunc(line, unicode.IsSpace)
	if i == -1 {
		return "", line
	}
	return line[0:i], skipWhitespaces(line[i:])
}

func parseStream(line string) (Stream, string) {
	stream := StreamUnknown

	// Optimize this!!
	if strings.HasPrefix(line, "stdout") {
		stream, line = StreamStdOut, line[len("stdout"):]
	} else if strings.HasPrefix(line, "stderr") {
		stream, line = StreamStdErr, line[len("stderr"):]
	}

	return stream, skipWhitespaces(line)
}

func parseFlag(line string) (Flag, string) {
	if len(line) == 0 {
		return FlagFull, line
	}

	var (
		b    = line[0]
		flag = FlagFull
	)

	switch b {
	case 'P':
		flag = FlagPartial
		line = line[1:]
	case 'F':
		line = line[1:]
	}

	return flag, skipWhitespaces(line)
}

func skipWhitespaces(b string) string {
	i := 0
	for i < len(b) {
		switch b[i] {
		case ' ':
			i++
		default:
			return b[i:]
		}
	}
	return b
}
