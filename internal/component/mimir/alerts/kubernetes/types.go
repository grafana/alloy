package alerts

import (
	"time"

	"github.com/grafana/alloy/internal/component/common/config"
	"github.com/grafana/alloy/internal/component/common/kubernetes"
	"github.com/grafana/alloy/syntax/alloytypes"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
)

type Arguments struct {
	Address          string                  `alloy:"address,attr"`
	HTTPClientConfig config.HTTPClientConfig `alloy:",squash"`
	SyncInterval     time.Duration           `alloy:"sync_interval,attr,optional"`

	TemplateFiles map[string]string `alloy:"template_files,attr,optional"`
	GlobalConfig  alloytypes.Secret `alloy:"global_config,attr"`

	AlertmanagerConfigSelector          kubernetes.LabelSelector `alloy:"alertmanagerconfig_selector,block,optional"`
	AlertmanagerConfigNamespaceSelector kubernetes.LabelSelector `alloy:"alertmanagerconfig_namespace_selector,block,optional"`
	AlertmanagerConfigMatcherStrategy   string                   `alloy:"alertmanagerconfig_matcher_strategy,attr,optional"`
}

var DefaultArguments = Arguments{
	SyncInterval:                      5 * time.Minute,
	HTTPClientConfig:                  config.DefaultHTTPClientConfig,
	AlertmanagerConfigMatcherStrategy: string(monitoringv1.OnNamespaceConfigMatcherStrategyType),
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
