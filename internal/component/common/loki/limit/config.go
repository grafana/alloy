package limit

import (
	"github.com/grafana/loki/pkg/util/flagext"
)

type Config struct {
	ReadlineRate        float64          `mapstructure:"readline_rate" yaml:"readline_rate" json:"readline_rate"`
	ReadlineBurst       int              `mapstructure:"readline_burst" yaml:"readline_burst" json:"readline_burst"`
	ReadlineRateEnabled bool             `mapstructure:"readline_rate_enabled,omitempty" yaml:"readline_rate_enabled,omitempty"  json:"readline_rate_enabled"`
	ReadlineRateDrop    bool             `mapstructure:"readline_rate_drop,omitempty" yaml:"readline_rate_drop,omitempty"  json:"readline_rate_drop"`
	MaxStreams          int              `mapstructure:"max_streams" yaml:"max_streams" json:"max_streams"`
	MaxLineSize         flagext.ByteSize `mapstructure:"max_line_size" yaml:"max_line_size" json:"max_line_size"`
	MaxLineSizeTruncate bool             `mapstructure:"max_line_size_truncate" yaml:"max_line_size_truncate" json:"max_line_size_truncate"`
}
