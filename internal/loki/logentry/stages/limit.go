package stages

const (
	ErrLimitStageInvalidRateOrBurst = "limit stage failed to parse rate or burst"
	ErrLimitStageByLabelMustDrop    = "When ratelimiting by label, drop must be true"
	MinReasonableMaxDistinctLabels  = 10000 // 80bytes per rate.Limiter ~ 1MiB memory
)

type LimitConfig struct {
	Rate              float64 `mapstructure:"rate"`
	Burst             int     `mapstructure:"burst"`
	Drop              bool    `mapstructure:"drop"`
	ByLabelName       string  `mapstructure:"by_label_name"`
	MaxDistinctLabels int     `mapstructure:"max_distinct_labels"`
}
