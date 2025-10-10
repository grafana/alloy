package stages

// Config Errors
const (
	ErrEmptyTemplateStageConfig = "template stage config cannot be empty"
	ErrTemplateSourceRequired   = "template source value is required"
)

// TemplateConfig configures template value extraction
type TemplateConfig struct {
	Source   string `mapstructure:"source"`
	Template string `mapstructure:"template"`
}
