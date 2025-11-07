package alerts

import (
	"github.com/grafana/alloy/internal/component/common/config"
	"github.com/grafana/alloy/internal/component/common/kubernetes"
	"github.com/grafana/alloy/syntax/alloytypes"
)

type Arguments struct {
	Address          string                  `alloy:"address,attr"`
	HTTPClientConfig config.HTTPClientConfig `alloy:",squash"`

	TemplateFiles map[string]string `alloy:"template_files,attr,optional"`
	GlobalConfig  alloytypes.Secret `alloy:"global_config,attr"`

	// TODO: Add an attr for the matcher strategy?

	AlertmanagerConfigSelector          kubernetes.LabelSelector `alloy:"alertmanagerconfig_selector,block,optional"`
	AlertmanagerConfigNamespaceSelector kubernetes.LabelSelector `alloy:"alertmanagerconfig_namespace_selector,block,optional"`
}

var DefaultArguments = Arguments{
	HTTPClientConfig: config.DefaultHTTPClientConfig,
}

// SetToDefault implements syntax.Defaulter.
func (args *Arguments) SetToDefault() {
	*args = DefaultArguments
}

// Validate implements syntax.Validator.
func (args *Arguments) Validate() error {
	// We must explicitly Validate because HTTPClientConfig is squashed and it won't run otherwise
	return args.HTTPClientConfig.Validate()
}
