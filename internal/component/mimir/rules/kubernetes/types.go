package rules

import (
	"fmt"
	"time"

	"github.com/grafana/alloy/internal/component/common/config"
	"github.com/grafana/alloy/internal/component/common/kubernetes"
)

type Arguments struct {
	Address              string                  `alloy:"address,attr"`
	TenantID             string                  `alloy:"tenant_id,attr,optional"`
	UseLegacyRoutes      bool                    `alloy:"use_legacy_routes,attr,optional"`
	PrometheusHTTPPrefix string                  `alloy:"prometheus_http_prefix,attr,optional"`
	HTTPClientConfig     config.HTTPClientConfig `alloy:",squash"`
	SyncInterval         time.Duration           `alloy:"sync_interval,attr,optional"`
	MimirNameSpacePrefix string                  `alloy:"mimir_namespace_prefix,attr,optional"`
	ExternalLabels       map[string]string       `alloy:"external_labels,attr,optional"`

	RuleSelector          kubernetes.LabelSelector `alloy:"rule_selector,block,optional"`
	RuleNamespaceSelector kubernetes.LabelSelector `alloy:"rule_namespace_selector,block,optional"`
}

var DefaultArguments = Arguments{
	SyncInterval:         5 * time.Minute,
	MimirNameSpacePrefix: "alloy",
	HTTPClientConfig:     config.DefaultHTTPClientConfig,
	PrometheusHTTPPrefix: "/prometheus",
}

// SetToDefault implements syntax.Defaulter.
func (args *Arguments) SetToDefault() {
	*args = DefaultArguments
}

// Validate implements syntax.Validator.
func (args *Arguments) Validate() error {
	if args.SyncInterval <= 0 {
		return fmt.Errorf("sync_interval must be greater than 0")
	}
	if args.MimirNameSpacePrefix == "" {
		return fmt.Errorf("mimir_namespace_prefix must not be empty")
	}

	// We must explicitly Validate because HTTPClientConfig is squashed and it won't run otherwise
	return args.HTTPClientConfig.Validate()
}
