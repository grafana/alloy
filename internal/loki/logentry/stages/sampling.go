package stages

const (
	ErrSamplingStageInvalidRate = "sampling stage failed to parse rate,Sampling Rate must be between 0.0 and 1.0, received %f"
)

// SamplingConfig contains the configuration for a samplingStage
type SamplingConfig struct {
	DropReason   *string `mapstructure:"drop_counter_reason"`
	SamplingRate float64 `mapstructure:"rate"`
}
