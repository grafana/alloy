package rules

import (
	"errors"
	"fmt"
	"slices"
	"time"

	promlabels "github.com/prometheus/prometheus/model/labels"

	"github.com/grafana/alloy/internal/component/common/config"
	"github.com/grafana/alloy/internal/component/common/kubernetes"
)

const AnnotationsSourceTenants = "monitoring.grafana.com/source_tenants"

var (
	// This should contain all valid match types for extra query matchers.
	validMatchTypes = []string{
		promlabels.MatchEqual.String(),
		promlabels.MatchNotEqual.String(),
		promlabels.MatchRegexp.String(),
		promlabels.MatchNotRegexp.String(),
	}
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
	ExtraQueryMatchers   *ExtraQueryMatchers     `alloy:"extra_query_matchers,block,optional"`

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
	if err := args.ExtraQueryMatchers.Validate(); err != nil {
		return err
	}

	// We must explicitly Validate because HTTPClientConfig is squashed and it won't run otherwise
	return args.HTTPClientConfig.Validate()
}

type ExtraQueryMatchers struct {
	Matchers []Matcher `alloy:"matcher,block,optional"`
}

func (e *ExtraQueryMatchers) Validate() error {
	if e == nil {
		return nil
	}
	var errs error
	for _, matcher := range e.Matchers {
		errs = errors.Join(errs, matcher.Validate())
	}
	return errs
}

type Matcher struct {
	Name           string `alloy:"name,attr"`
	Value          string `alloy:"value,attr,optional"`
	ValueFromLabel string `alloy:"value_from_label,attr,optional"`
	MatchType      string `alloy:"match_type,attr"`
}

func (m Matcher) Validate() error {
	if !slices.Contains(validMatchTypes, m.MatchType) {
		return fmt.Errorf("invalid match type: %q", m.MatchType)
	}
	// Check that exactly one value source is provided
	valueSourceCount := 0
	for _, field := range []string{m.Value, m.ValueFromLabel} {
		if field != "" {
			valueSourceCount++
		}
	}
	if valueSourceCount != 1 {
		return fmt.Errorf("exactly one of 'value' or 'value_from_label' must be provided, got %d", valueSourceCount)
	}
	return nil
}
