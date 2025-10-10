package file

import (
	"flag"
	"time"
)

const (
	FilenameLabel = "filename"
)

// Config describes behavior for Target
type Config struct {
	SyncPeriod time.Duration `mapstructure:"sync_period" yaml:"sync_period"`
	Stdin      bool          `mapstructure:"stdin" yaml:"stdin"`
}

// RegisterFlags with prefix registers flags where every name is prefixed by
// prefix. If prefix is a non-empty string, prefix should end with a period.
func (cfg *Config) RegisterFlagsWithPrefix(prefix string, f *flag.FlagSet) {
	f.DurationVar(&cfg.SyncPeriod, prefix+"target.sync-period", 10*time.Second, "Period to resync directories being watched and files being tailed.")
	f.BoolVar(&cfg.Stdin, prefix+"stdin", false, "Set to true to pipe logs to promtail.")
}

// RegisterFlags register flags.
func (cfg *Config) RegisterFlags(flags *flag.FlagSet) {
	cfg.RegisterFlagsWithPrefix("", flags)
}

type WatchConfig struct {
	MinPollFrequency time.Duration `mapstructure:"min_poll_frequency" yaml:"min_poll_frequency"`
	MaxPollFrequency time.Duration `mapstructure:"max_poll_frequency" yaml:"max_poll_frequency"`
}

var DefaultWatchConfig = WatchConfig{
	MinPollFrequency: 250 * time.Millisecond,
	MaxPollFrequency: 250 * time.Millisecond,
}

// RegisterFlags with prefix registers flags where every name is prefixed by
// prefix. If prefix is a non-empty string, prefix should end with a period.
func (cfg *WatchConfig) RegisterFlagsWithPrefix(prefix string, f *flag.FlagSet) {
	d := DefaultWatchConfig

	f.DurationVar(&cfg.MinPollFrequency, prefix+"min_poll_frequency", d.MinPollFrequency, "Minimum period to poll for file changes")
	f.DurationVar(&cfg.MaxPollFrequency, prefix+"max_poll_frequency", d.MaxPollFrequency, "Maximum period to poll for file changes")
}

// RegisterFlags register flags.
func (cfg *WatchConfig) RegisterFlags(flags *flag.FlagSet) {
	cfg.RegisterFlagsWithPrefix("", flags)
}
