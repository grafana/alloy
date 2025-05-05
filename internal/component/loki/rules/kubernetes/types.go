package rules

import (
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/prometheus/prometheus/model/labels"

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
	ExtraQueryMatchers  *ExtraQueryMatchers     `alloy:"extra_query_matchers,block,optional"`

	RuleSelector          kubernetes.LabelSelector `alloy:"rule_selector,block,optional"`
	RuleNamespaceSelector kubernetes.LabelSelector `alloy:"rule_namespace_selector,block,optional"`
}

var (
	// This should contain all valid match types for extra query matchers.
	validMatchTypes = []string{
		labels.MatchEqual.String(),
		labels.MatchNotEqual.String(),
		labels.MatchRegexp.String(),
		labels.MatchNotRegexp.String(),
	}
)

var DefaultArguments = Arguments{
	SyncInterval:        30 * time.Second,
	LokiNameSpacePrefix: "alloy",
	HTTPClientConfig:    config.DefaultHTTPClientConfig,
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
	if args.LokiNameSpacePrefix == "" {
		return fmt.Errorf("loki_namespace_prefix must not be empty")
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
	Name      string `alloy:"name,attr"`
	Value     string `alloy:"value,attr"`
	MatchType string `alloy:"match_type,attr"`
}

func (m Matcher) Validate() error {
	if !slices.Contains(validMatchTypes, m.MatchType) {
		return fmt.Errorf("invalid match type: %q", m.MatchType)
	}
	return nil
}
