package stages

type EventLogMessageConfig struct {
	Source            *string `mapstructure:"source"`
	DropInvalidLabels bool    `mapstructure:"drop_invalid_labels"`
	OverwriteExisting bool    `mapstructure:"overwrite_existing"`
}
