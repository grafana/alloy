package servicemonitors

import "github.com/grafana/alloy/internal/component/prometheus/operator"

type Arguments struct {
	Arguments operator.Arguments `alloy:",squash"`

	// AllowArbitraryFileAccess allows arbitrary file access on ServiceMonitor endpoints.
	AllowArbitraryFileAccess bool `alloy:"allow_arbitrary_file_access,attr,optional"`
}

var DefaultArguments = Arguments{
	Arguments:                operator.DefaultArguments,
	AllowArbitraryFileAccess: false,
}

// SetToDefault implements syntax.Defaulter.
func (args *Arguments) SetToDefault() {
	*args = DefaultArguments
}

// Validate implements syntax.Validator.
func (args *Arguments) Validate() error {
	return args.Arguments.Validate()
}
