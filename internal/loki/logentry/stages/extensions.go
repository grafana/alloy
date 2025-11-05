package stages

import (
	"github.com/grafana/loki/v3/pkg/util/flagext"
)

const (
	RFC3339Nano         = "RFC3339Nano"
	MaxPartialLinesSize = 100 // Max buffer size to hold partial lines.
)

// CriConfig contains the configuration for the cri stage
type CriConfig struct {
	MaxPartialLines            int              `mapstructure:"max_partial_lines"`
	MaxPartialLineSize         flagext.ByteSize `mapstructure:"max_partial_line_size"`
	MaxPartialLineSizeTruncate bool             `mapstructure:"max_partial_line_size_truncate"`
}
