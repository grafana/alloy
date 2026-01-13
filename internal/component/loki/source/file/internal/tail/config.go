package tail

import (
	"time"

	"golang.org/x/text/encoding"
)

// Config holds configuration for tailing a file.
type Config struct {
	// Filename is the path to the file to tail.
	Filename string
	// Offset is the byte offset in the file where tailing should start.
	// If 0, tailing starts from the beginning of the file.
	Offset int64

	// FIXME: Update text
	// Decoder is an optional text decoder for non-UTF-8 encoded files.
	// If the file is not UTF-8, the tailer must use the correct decoder
	// or the output text may be corrupted. For example, if the file is
	// "UTF-16 LE" encoded, the tailer would not separate new lines properly
	// and the output could appear as garbled characters.
	Encoding encoding.Encoding

	// WatcherConfig controls how the file system is polled for changes.
	WatcherConfig WatcherConfig
}

// WatcherConfig controls the polling behavior for detecting file system events.
type WatcherConfig struct {
	// MinPollFrequency and MaxPollFrequency specify the polling frequency range
	// for detecting file system events. The actual polling frequency will vary
	// within this range based on backoff behavior.
	MinPollFrequency, MaxPollFrequency time.Duration
}

// defaultWatcherConfig holds the default polling configuration used when
// WatcherConfig is not explicitly provided in Config.
var defaultWatcherConfig = WatcherConfig{
	MinPollFrequency: 250 * time.Millisecond,
	MaxPollFrequency: 250 * time.Millisecond,
}
