package resource_attribute_config

// Configures whether a resource attribute should be enabled or not.
type ResourceAttributeConfig struct {
	Enabled bool `river:"enabled,attr"`
}

func (r ResourceAttributeConfig) Convert() map[string]interface{} {
	return map[string]interface{}{
		"enabled": r.Enabled,
	}
}
