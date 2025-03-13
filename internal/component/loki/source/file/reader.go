package file

// This code is copied from loki/promtail@a8d5815510bd959a6dd8c176a5d9fd9bbfc8f8b5.
// This code accommodates the tailer and decompressor implementations as readers.

// reader contains the set of methods the loki.source.file component uses.
type reader interface {
	Stop()
	IsRunning() bool
	Path() string
	MarkPositionAndSize() error
}
