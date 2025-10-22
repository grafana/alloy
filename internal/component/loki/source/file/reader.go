package file

import "context"

// This code is adopted from loki/promtail@a8d5815510bd959a6dd8c176a5d9fd9bbfc8f8b5.
// This code accommodates the tailer and decompressor implementations as readers.

// reader contains the set of methods the loki.source.file component uses.
type reader interface {
	// Run will start the reader job and exit if context is canceled or
	// if job finished. It's not safe to call run on the same reader from different
	// goroutines.
	Run(ctx context.Context)
	IsRunning() bool
}
