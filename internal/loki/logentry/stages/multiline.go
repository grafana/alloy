package stages

const (
	ErrMultilineStageEmptyConfig        = "multiline stage config must define `firstline` regular expression"
	ErrMultilineStageInvalidRegex       = "multiline stage first line regex compilation error: %v"
	ErrMultilineStageInvalidMaxWaitTime = "multiline stage `max_wait_time` parse error: %v"
)

// MultilineConfig contains the configuration for a multilineStage
type MultilineConfig struct {
	Expression  *string `mapstructure:"firstline"`
	MaxLines    *uint64 `mapstructure:"max_lines"`
	MaxWaitTime *string `mapstructure:"max_wait_time"`
}
