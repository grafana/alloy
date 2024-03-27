package rules

import (
	"fmt"
	"time"

	"github.com/grafana/alloy/internal/component/common/config"
	"github.com/grafana/alloy/internal/component/common/kubernetes"
)

type Arguments struct {
	Address             string                  `alloy:"address,attr"`
	TenantID            string                  `alloy:"tenant_id,attr,optional"`
	UseLegacyRoutes     bool                    `alloy:"use_legacy_routes,attr,optional"`
	HTTPClientConfig    config.HTTPClientConfig `alloy:",squash"`
	SyncInterval        time.Duration           `alloy:"sync_interval,attr,optional"`
	LokiNameSpacePrefix string                  `alloy:"loki_namespace_prefix,attr,optional"`

	RuleSelector          kubernetes.LabelSelector `alloy:"rule_selector,block,optional"`
	RuleNamespaceSelector kubernetes.LabelSelector `alloy:"rule_namespace_selector,block,optional"`
}

var DefaultArguments = Arguments{
	SyncInterval:        30 * time.Second,
	LokiNameSpacePrefix: "alloy",
	HTTPClientConfig:    config.DefaultHTTPClientConfig,
}

// SetToDefault implements river.Defaulter.
func (args *Arguments) SetToDefault() {
	*args = DefaultArguments
}

// Validate implements river.Validator.
func (args *Arguments) Validate() error {
	if args.SyncInterval <= 0 {
		return fmt.Errorf("sync_interval must be greater than 0")
	}
	if args.LokiNameSpacePrefix == "" {
		return fmt.Errorf("loki_namespace_prefix must not be empty")
	}

	// We must explicitly Validate because HTTPClientConfig is squashed and it won't run otherwise
	return args.HTTPClientConfig.Validate()
}
