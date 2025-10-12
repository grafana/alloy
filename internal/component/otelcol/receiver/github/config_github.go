package github

import (
	"errors"
	"time"
	"unsafe"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/githubreceiver"

	"go.opentelemetry.io/collector/scraper/scraperhelper"
)

type GithubConfig struct {
	InitialDelay       time.Duration            `alloy:"initial_delay,attr,optional"`
	CollectionInterval time.Duration            `alloy:"collection_interval,attr,optional"`
	Scrapers           map[string]ScraperConfig `alloy:"scraper,block"`
}

func (args GithubConfig) Convert() githubreceiver.Config {
	convertedScrapers := make(map[string]interface{}, len(args.Scrapers))
	for name, scraper := range args.Scrapers {
		convertedScrapers[name] = scraper.Convert()
	}

	config := githubreceiver.Config{
		ControllerConfig: scraperhelper.ControllerConfig{
			InitialDelay:       args.InitialDelay,
			CollectionInterval: args.CollectionInterval,
		},
	}

	*(*map[string]interface{})(unsafe.Pointer(&config.Scrapers)) = convertedScrapers

	return config
}

func (args *GithubConfig) SetToDefault() {
	if args.InitialDelay == 0 {
		args.InitialDelay = 0
	}
	if args.CollectionInterval == 0 {
		args.CollectionInterval = 60 * time.Second
	}

	for _, scraper := range args.Scrapers {
		scraper.SetToDefault()
	}
}

func (args GithubConfig) Validate() error {
	for _, scraper := range args.Scrapers {
		if err := scraper.Validate(); err != nil {
			return err
		}
	}

	return nil
}

type ScraperConfig struct {
	GithubOrg   string        `alloy:"github_org,attr"`
	SearchQuery string        `alloy:"search_query,attr"`
	Endpoint    string        `alloy:"endpoint,attr,optional"`
	Auth        AuthConfig    `alloy:"auth,block"`
	Metrics     MetricsConfig `alloy:"metrics,block,optional"`
}

func (sc ScraperConfig) Convert() map[string]interface{} {
	return map[string]interface{}{
		"github_org":   sc.GithubOrg,
		"search_query": sc.SearchQuery,
		"endpoint":     sc.Endpoint,
		"auth":         sc.Auth.Convert(),
		"metrics":      sc.Metrics.Convert(),
	}
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
	Authenticator string `alloy:"authenticator,attr"`
}

func (ac AuthConfig) Convert() map[string]interface{} {
	return map[string]interface{}{
		"authenticator": ac.Authenticator,
	}
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
