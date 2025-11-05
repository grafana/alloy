package stages

// DropConfig contains the configuration for a dropStage
type DropConfig struct {
	DropReason *string     `mapstructure:"drop_counter_reason"`
	Source     interface{} `mapstructure:"source"`
	Value      *string     `mapstructure:"value"`
	Separator  *string     `mapstructure:"separator"`
	Expression *string     `mapstructure:"expression"`
	OlderThan  *string     `mapstructure:"older_than"`
	LongerThan *string     `mapstructure:"longer_than"`
}
