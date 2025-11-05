package stages

// Config Errors
const (
	ErrEmptyReplaceStageConfig = "empty replace stage configuration"
	ErrEmptyReplaceStageSource = "empty source in replace stage"
)

// ReplaceConfig contains a regexStage configuration
type ReplaceConfig struct {
	Expression string  `mapstructure:"expression"`
	Source     *string `mapstructure:"source"`
	Replace    string  `mapstructure:"replace"`
}
