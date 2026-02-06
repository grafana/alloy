package resource_attribute_config

// Configures whether a resource attribute should be enabled or not.
type ResourceAttributeConfig struct {
	Enabled bool `alloy:"enabled,attr"`
}

func (r ResourceAttributeConfig) Convert() map[string]any {
	return map[string]any{
		"enabled": r.Enabled,
	}
}
