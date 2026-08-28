package positions

import (
	"flag"
	"time"
)

// Config describes where to get position information from.
type Config struct {
	SyncPeriod        time.Duration `mapstructure:"sync_period" yaml:"sync_period"`
	PositionsFile     string        `mapstructure:"filename" yaml:"filename"`
	IgnoreInvalidYaml bool          `mapstructure:"ignore_invalid_yaml" yaml:"ignore_invalid_yaml"`
	ReadOnly          bool          `mapstructure:"-" yaml:"-"`
}

// RegisterFlags with prefix registers flags where every name is prefixed by
// prefix. If prefix is a non-empty string, prefix should end with a period.
func (cfg *Config) RegisterFlagsWithPrefix(prefix string, f *flag.FlagSet) {
	f.DurationVar(&cfg.SyncPeriod, prefix+"positions.sync-period", 10*time.Second, "Period with this to sync the position file.")
	f.StringVar(&cfg.PositionsFile, prefix+"positions.file", "/var/log/positions.yaml", "Location to read/write positions from.")
	f.BoolVar(&cfg.IgnoreInvalidYaml, prefix+"positions.ignore-invalid-yaml", false, "whether to ignore & later overwrite positions files that are corrupted")
}

// RegisterFlags register flags.
func (cfg *Config) RegisterFlags(flags *flag.FlagSet) {
	cfg.RegisterFlagsWithPrefix("", flags)
}
