package stages

// Config Errors
const (
	ErrEmptyOutputStageConfig = "output stage config cannot be empty"
	ErrOutputSourceRequired   = "output source value is required if output is specified"
)

// OutputConfig configures output value extraction
type OutputConfig struct {
	Source string `mapstructure:"source"`
}
