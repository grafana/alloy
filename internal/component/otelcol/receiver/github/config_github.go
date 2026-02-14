package github

import (
	"errors"

	"github.com/grafana/alloy/internal/component/otelcol/auth"
)

// scraper
type ScraperConfig struct {
	GithubOrg        string        `alloy:"github_org,attr"`
	Endpoint         string        `alloy:"endpoint,attr,optional"`
	SearchQuery      string        `alloy:"search_query,attr,optional"`
	ConcurrencyLimit int           `alloy:"concurrency_limit,attr,optional"`
	Auth             AuthConfig    `alloy:"auth,block"`
	Metrics          MetricsConfig `alloy:"metrics,block,optional"`
}

func (sc ScraperConfig) Convert() (map[string]interface{}, error) {
	authConfig, err := sc.Auth.Convert()
	if err != nil {
		return nil, err
	}

	// has to return a map[string]interface{}
	// the scraper config is nested within an internal package
	// so we can't directly convert it to the expected otelcol config struct
	return map[string]interface{}{
		"github_org":        sc.GithubOrg,
		"endpoint":          sc.Endpoint,
		"search_query":      sc.SearchQuery,
		"concurrency_limit": sc.ConcurrencyLimit,
		"auth":              authConfig,
		"metrics":           sc.Metrics.Convert(),
	}, nil
}

func (sc *ScraperConfig) SetToDefault() {

	if sc.Metrics == (MetricsConfig{}) {
		sc.Metrics.SetToDefault()
	}
}

func (sc *ScraperConfig) Validate() error {
	if sc.GithubOrg == "" {
		return errors.New("github_org is required")
	}
	return nil
}

type AuthConfig struct {
	Authenticator *auth.Handler `alloy:"authenticator,attr"`
}

func (ac AuthConfig) Convert() (map[string]interface{}, error) {
	if ac.Authenticator == nil {
		return nil, nil
	}

	ext, err := ac.Authenticator.GetExtension(auth.Client)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"authenticator": ext.ID.String(),
	}, nil
}

type MetricConfig struct {
	Enabled bool `alloy:"enabled,attr"`
}

type MetricsConfig struct {
	VCSChangeCount          MetricConfig `alloy:"vcs.change.count,block,optional"`
	VCSChangeDuration       MetricConfig `alloy:"vcs.change.duration,block,optional"`
	VCSChangeTimeToApproval MetricConfig `alloy:"vcs.change.time_to_approval,block,optional"`
	VCSChangeTimeToMerge    MetricConfig `alloy:"vcs.change.time_to_merge,block,optional"`
	VCSRefCount             MetricConfig `alloy:"vcs.ref.count,block,optional"`
	VCSRefLinesDelta        MetricConfig `alloy:"vcs.ref.lines_delta,block,optional"`
	VCSRefRevisionsDelta    MetricConfig `alloy:"vcs.ref.revisions_delta,block,optional"`
	VCSRefTime              MetricConfig `alloy:"vcs.ref.time,block,optional"`
	VCSRepositoryCount      MetricConfig `alloy:"vcs.repository.count,block,optional"`
	VCSContributorCount     MetricConfig `alloy:"vcs.contributor.count,block,optional"`
}

func (m *MetricsConfig) Convert() map[string]interface{} {
	if m == nil {
		return nil
	}

	return map[string]interface{}{
		"vcs.change.count":            m.VCSChangeCount.Convert(),
		"vcs.change.duration":         m.VCSChangeDuration.Convert(),
		"vcs.change.time_to_approval": m.VCSChangeTimeToApproval.Convert(),
		"vcs.change.time_to_merge":    m.VCSChangeTimeToMerge.Convert(),
		"vcs.ref.count":               m.VCSRefCount.Convert(),
		"vcs.ref.lines_delta":         m.VCSRefLinesDelta.Convert(),
		"vcs.ref.revisions_delta":     m.VCSRefRevisionsDelta.Convert(),
		"vcs.ref.time":                m.VCSRefTime.Convert(),
		"vcs.repository.count":        m.VCSRepositoryCount.Convert(),
		"vcs.contributor.count":       m.VCSContributorCount.Convert(),
	}
}

func (m *MetricConfig) Convert() map[string]interface{} {
	if m == nil {
		return nil
	}

	return map[string]interface{}{
		"enabled": m.Enabled,
	}
}

func (mc *MetricsConfig) SetToDefault() {
	*mc = MetricsConfig{
		VCSChangeCount:          MetricConfig{Enabled: true},
		VCSChangeDuration:       MetricConfig{Enabled: true},
		VCSChangeTimeToApproval: MetricConfig{Enabled: true},
		VCSChangeTimeToMerge:    MetricConfig{Enabled: true},
		VCSRefCount:             MetricConfig{Enabled: true},
		VCSRefLinesDelta:        MetricConfig{Enabled: true},
		VCSRefRevisionsDelta:    MetricConfig{Enabled: true},
		VCSRefTime:              MetricConfig{Enabled: true},
		VCSRepositoryCount:      MetricConfig{Enabled: true},
		VCSContributorCount:     MetricConfig{Enabled: false},
	}
}

// Webhook
type WebhookConfig struct {
	Endpoint          string            `alloy:"endpoint,attr,optional"`
	Path              string            `alloy:"path,attr,optional"`
	HealthPath        string            `alloy:"health_path,attr,optional"`
	Secret            string            `alloy:"secret,attr,optional"`
	RequiredHeaders   map[string]string `alloy:"required_headers,attr,optional"`
	ServiceName       string            `alloy:"service_name,attr,optional"`
	IncludeSpanEvents bool              `alloy:"include_span_events,attr,optional"`
}

func (wc WebhookConfig) Convert() map[string]interface{} {
	return map[string]interface{}{
		"endpoint":            wc.Endpoint,
		"path":                wc.Path,
		"health_path":         wc.HealthPath,
		"secret":              wc.Secret,
		"required_headers":    wc.RequiredHeaders,
		"service_name":        wc.ServiceName,
		"include_span_events": wc.IncludeSpanEvents,
	}
}

func (wc *WebhookConfig) SetToDefault() {
	if wc.Endpoint == "" {
		wc.Endpoint = "localhost:8080"
	}

	if wc.Path == "" {
		wc.Path = "/events"
	}

	if wc.HealthPath == "" {
		wc.HealthPath = "/health"
	}
}
