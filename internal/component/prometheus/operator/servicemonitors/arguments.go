package servicemonitors

import "github.com/grafana/alloy/internal/component/prometheus/operator"

type Arguments struct {
	Arguments operator.Arguments `alloy:",squash"`

	// DisallowArbitraryFileAccess disallows arbitrary file access on ServiceMonitor endpoints.
	DisallowArbitraryFileAccess bool `alloy:"disallow_arbitrary_file_access,attr,optional"`
}

var DefaultArguments = Arguments{
	Arguments:                   operator.DefaultArguments,
	DisallowArbitraryFileAccess: true,
}

// SetToDefault implements syntax.Defaulter.
func (args *Arguments) SetToDefault() {
	*args = DefaultArguments
}

// Validate implements syntax.Validator.
func (args *Arguments) Validate() error {
	return args.Arguments.Validate()
}

func (args Arguments) OperatorArguments() operator.Arguments {
	return args.Arguments
}

func (args Arguments) ServiceMonitorDisallowArbitraryFileAccess() bool {
	return args.DisallowArbitraryFileAccess
}
