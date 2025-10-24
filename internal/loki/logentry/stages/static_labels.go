package stages

const (
	// ErrEmptyStaticLabelStageConfig error returned if config is empty
	ErrEmptyStaticLabelStageConfig = "static_labels stage config cannot be empty"
)

// StaticLabelConfig is a slice of static-labels to be included
type StaticLabelConfig map[string]*string
