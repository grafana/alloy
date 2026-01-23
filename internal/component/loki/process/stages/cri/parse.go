package cri

import (
	"bytes"
	"unicode"
)

type Flag int8

const (
	FlagFull Flag = iota
	FlagPartial
)

func (f Flag) String() string {
	switch f {
	case FlagFull:
		return "F"
	case FlagPartial:
		return "P"
	default:
		return ""
	}
}

type Stream int8

const (
	StreamUnknown Stream = iota
	StreamStdOut
	StreamStdErr
)

func (s Stream) String() string {
	switch s {
	case StreamStdOut:
		return "stdout"
	case StreamStdErr:
		return "stderr"
	default:
		return ""
	}
}

type Parsed struct {
	Timestamp string
	Stream    Stream
	Flag      Flag
	Content   string
}

func ParseCRI(line []byte) (Parsed, bool) {
	var (
		timestamp []byte
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
		Timestamp: string(timestamp),
		Stream:    stream,
		Flag:      flag,
		Content:   string(line),
	}, true
}

func parseTimestamp(line []byte) ([]byte, []byte) {
	i := bytes.IndexFunc(line, unicode.IsSpace)
	if i == -1 {
		return nil, line
	}
	return line[0:i], skipWhitespaces(line[i:])
}

func parseStream(line []byte) (Stream, []byte) {
	stream := StreamUnknown

	// Optimize this!!
	if bytes.HasPrefix(line, []byte("stdout")) {
		stream, line = StreamStdOut, line[len("stdout"):]
	} else if bytes.HasPrefix(line, []byte("stderr")) {
		stream, line = StreamStdErr, line[len("stderr"):]
	}

	return stream, skipWhitespaces(line)
}

func parseFlag(line []byte) (Flag, []byte) {
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

// Not sure if we care about unicode
// FIXME: I think it's enough to skip only one..
func skipWhitespaces(b []byte) []byte {
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
