package stages

// Config Errors
const (
	ErrExpressionsRequired  = "JMES expression is required"
	ErrCouldNotCompileJMES  = "could not compile JMES expression"
	ErrEmptyJSONStageConfig = "empty json stage configuration"
	ErrEmptyJSONStageSource = "empty source"
	ErrMalformedJSON        = "malformed json"
)

// JSONConfig represents a JSON Stage configuration
type JSONConfig struct {
	Expressions   map[string]string `mapstructure:"expressions"`
	Source        *string           `mapstructure:"source"`
	DropMalformed bool              `mapstructure:"drop_malformed"`
}
