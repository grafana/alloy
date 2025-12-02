package tailv2

import (
	"time"

	"golang.org/x/text/encoding"
)

type Config struct {
	Filename string
	Offset   int64

	// Change the decoder if the file is not UTF-8.
	// If the tailer doesn't use the right decoding, the output text may be gibberish.
	// For example, if the file is "UTF-16 LE" encoded, the tailer would not separate
	// the new lines properly and the output could come out as chinese characters.
	Decoder *encoding.Decoder

	WatcherConfig WatcherConfig
}

type WatcherConfig struct {
	// MinPollFrequency and MaxPollFrequency specify how frequently a
	// PollingFileWatcher should poll the file.
	//
	// Watcher starts polling at MinPollFrequency, and will
	// exponentially increase the polling frequency up to MaxPollFrequency if no
	// new entries are found. The polling frequency is reset to MinPollFrequency
	// whenever the file changes.
	MinPollFrequency, MaxPollFrequency time.Duration
}

// DefaultWatcherConfig holds default values for WatcherConfig
var DefaultWatcherConfig = WatcherConfig{
	MinPollFrequency: 250 * time.Millisecond,
	MaxPollFrequency: 250 * time.Millisecond,
}
