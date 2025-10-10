package stages

// Config Errors
const (
	ErrMappingRequired        = "logfmt mapping is required"
	ErrEmptyLogfmtStageConfig = "empty logfmt stage configuration"
	ErrEmptyLogfmtStageSource = "empty source"
)

// LogfmtConfig represents a logfmt Stage configuration
type LogfmtConfig struct {
	Mapping map[string]string `mapstructure:"mapping"`
	Source  *string           `mapstructure:"source"`
}
