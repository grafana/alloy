package stages

const (
	// ErrEmptyLabelAllowStageConfig error returned if config is empty
	ErrEmptyLabelAllowStageConfig = "labelallow stage config cannot be empty"
)

// labelallowConfig is a slice of labels to be included
type LabelAllowConfig []string
