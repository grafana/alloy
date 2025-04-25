package alerts

import (
	"errors"
	"fmt"
	"slices"

	promlabels "github.com/prometheus/prometheus/model/labels"

	"github.com/grafana/alloy/internal/component/common/config"
	"github.com/grafana/alloy/internal/component/common/kubernetes"
)

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
	Address          string                  `alloy:"address,attr"`
	HTTPClientConfig config.HTTPClientConfig `alloy:",squash"`

	TemplateFiles map[string]string `alloy:"template_files,attr,optional"`
	GlobalConfig  string            `alloy:"global_config,attr,optional"`

	// TODO: Add an attr for the matcher strategy?

	// TODO: Should we have a mimir_namespace_prefix argument?
	// MimirNameSpacePrefix string                  `alloy:"mimir_namespace_prefix,attr,optional"`

	// TODO: Do we need a tenant ID?
	// TenantID string `alloy:"tenant_id,attr,optional"`

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

// TODO: Reuse the same type as mimir.rules.kubernetes
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

// TODO: Reuse the same type as mimir.rules.kubernetes
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
