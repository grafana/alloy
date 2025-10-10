package stages

const (
	ErrEmptyTimestampStageConfig = "timestamp stage config cannot be empty"
	ErrTimestampSourceRequired   = "timestamp source value is required if timestamp is specified"
	ErrTimestampFormatRequired   = "timestamp format is required"
	ErrInvalidLocation           = "invalid location specified: %v"
	ErrInvalidActionOnFailure    = "invalid action on failure (supported values are %v)"
	ErrTimestampSourceMissing    = "extracted data did not contain a timestamp"
	ErrTimestampConversionFailed = "failed to convert extracted time to string"
	ErrTimestampParsingFailed    = "failed to parse time"

	Unix   = "Unix"
	UnixMs = "UnixMs"
	UnixUs = "UnixUs"
	UnixNs = "UnixNs"

	TimestampActionOnFailureSkip    = "skip"
	TimestampActionOnFailureFudge   = "fudge"
	TimestampActionOnFailureDefault = TimestampActionOnFailureFudge
)

var (
	TimestampActionOnFailureOptions = []string{TimestampActionOnFailureSkip, TimestampActionOnFailureFudge}
)

// TimestampConfig configures timestamp extraction
type TimestampConfig struct {
	Source          string   `mapstructure:"source"`
	Format          string   `mapstructure:"format"`
	FallbackFormats []string `mapstructure:"fallback_formats"`
	Location        *string  `mapstructure:"location"`
	ActionOnFailure *string  `mapstructure:"action_on_failure"`
}
