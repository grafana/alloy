package wal

import (
	"time"
)

const (
	DefaultMaxSegmentAge = time.Hour
)

// DefaultWatchConfig is the opinionated defaults for operating the Watcher.
var DefaultWatchConfig = WatchConfig{
	MinReadFrequency: 250 * time.Millisecond,
	MaxReadFrequency: time.Second,
	DrainTimeout:     15 * time.Second,
}

// Config contains all WAL-related settings.
type Config struct {
	// Whether WAL-support should be enabled.
	//
	// WAL support is a WIP. Do not enable in production setups until https://github.com/grafana/loki/issues/8197
	// is finished.
	Enabled bool

	// Path where the WAL is written to.
	Dir string

	// MaxSegmentAge is threshold at which a WAL segment is considered old enough to be cleaned up. Default: 1h.
	//
	// Note that this functionality will likely be deprecated in favour of a programmatic cleanup mechanism.
	MaxSegmentAge time.Duration

	// WatchConfig configures the backoff retry used by a WAL watcher when reading from segments not via
	// the notification channel.
	WatchConfig WatchConfig
}

// WatchConfig allows the user to configure the Watcher.
//
// For the read frequency settings, the Watcher polls the WAL for new records with two mechanisms: First, it gets
// notified by the Writer when the WAL is written; also, it has a timer that gets fired every so often. This last
// one, implements and exponential back-off strategy to prevent the Watcher from doing read too often, if there's no new
// data.
type WatchConfig struct {
	// MinReadFrequency controls the minimum read frequency the Watcher polls the WAL for new records. If the poll is successful,
	// the frequency will remain the same. If not, it will be incremented using an exponential backoff.
	MinReadFrequency time.Duration

	// MaxReadFrequency controls the maximum read frequency the Watcher polls the WAL for new records. As mentioned above
	// it caps the polling frequency to a maximum, to prevent to exponential backoff from making it too high.
	MaxReadFrequency time.Duration

	// DrainTimeout is the maximum amount of time that the Watcher can spend draining the remaining segments in the WAL.
	// After that time, the Watcher is stopped immediately, dropping all the work in process.
	DrainTimeout time.Duration
}

// UnmarshalYAML implement YAML Unmarshaler
func (c *Config) UnmarshalYAML(unmarshal func(interface{}) error) error {
	// Apply defaults
	c.MaxSegmentAge = DefaultMaxSegmentAge
	c.WatchConfig = DefaultWatchConfig
	type plain Config
	return unmarshal((*plain)(c))
}
