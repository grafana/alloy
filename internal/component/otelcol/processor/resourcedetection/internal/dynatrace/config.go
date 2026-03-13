package dynatrace

import (
	"github.com/grafana/alloy/syntax"
)

const Name = "dynatrace"

type Config struct {
}

// DefaultArguments holds default settings for Config.
var DefaultArguments = Config{}

var _ syntax.Defaulter = (*Config)(nil)

// SetToDefault implements syntax.Defaulter.
func (args *Config) SetToDefault() {
	*args = DefaultArguments
}

func (args Config) Convert() map[string]any {
	return map[string]any{}
}
