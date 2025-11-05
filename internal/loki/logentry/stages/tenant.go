package stages

const (
	ErrTenantStageEmptyLabelSourceOrValue        = "label, source or value config are required"
	ErrTenantStageConflictingLabelSourceAndValue = "label, source and value are mutually exclusive: you should set source, value or label but not all"
)

type TenantConfig struct {
	Label  string `mapstructure:"label"`
	Source string `mapstructure:"source"`
	Value  string `mapstructure:"value"`
}
