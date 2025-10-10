package stages

// Config Errors
const (
	ErrExpressionRequired    = "expression is required"
	ErrCouldNotCompileRegex  = "could not compile regular expression"
	ErrEmptyRegexStageConfig = "empty regex stage configuration"
	ErrEmptyRegexStageSource = "empty source"
)

// RegexConfig contains a regexStage configuration
type RegexConfig struct {
	Expression string  `mapstructure:"expression"`
	Source     *string `mapstructure:"source"`
}
