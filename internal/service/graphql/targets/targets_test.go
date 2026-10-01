package targets_test

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/service/graphql/graph/model"
	"github.com/grafana/alloy/internal/service/graphql/targets"
)

func TestTargetsExtractsDirectDiscoveryTargets(t *testing.T) {
	info := &component.Info{Exports: directTargetExports{
		Targets: []discovery.Target{discovery.NewTargetFromMap(map[string]string{"job": "api"})},
	}}

	got := targets.FromComponent(info)

	require.Len(t, got, 1)
	require.Equal(t, []model.LabelPair{{Name: "job", Value: "api"}}, got[0].Labels)
}

func TestTargetsExtractsNestedDiscoveryTargets(t *testing.T) {
	info := &component.Info{Exports: nestedTargetExports{
		Nested: nestedTargets{Targets: []discovery.Target{discovery.NewTargetFromMap(map[string]string{"instance": "localhost:9090"})}},
	}}

	got := targets.FromComponent(info)

	require.Len(t, got, 1)
	require.Equal(t, "instance", got[0].Labels[0].Name)
}

func TestTargetsReturnsEmptyForComponentsWithoutTargets(t *testing.T) {
	info := &component.Info{Exports: struct{ Value string }{Value: "no targets"}}

	got := targets.FromComponent(info)

	require.Empty(t, got)
}

func TestTargetsUsesComponentOwnedScrapeRuntime(t *testing.T) {
	lastAttempt := time.Date(2026, time.October, 1, 12, 30, 0, 0, time.UTC)
	info := &component.Info{
		Component: &targetProvider{targets: []component.TargetInfo{{
			Labels:        map[string]string{"__address__": "example.com:9090", "__param_token": "secret", "job": "api"},
			NonMetaLabels: map[string]string{"job": "api"},
			Hash:          10,
			NonMetaHash:   20,
			Scrape: &component.ScrapeTargetInfo{
				URL:          "https://example.com:9090/metrics",
				Health:       component.ScrapeTargetHealthUp,
				LastAttempt:  lastAttempt,
				LastDuration: 250 * time.Millisecond,
			},
		}}},
		Exports: directTargetExports{
			Targets: []discovery.Target{discovery.NewTargetFromMap(map[string]string{"job": "fallback"})},
		},
	}

	got := targets.FromComponent(info)

	require.Len(t, got, 1)
	require.Equal(t, "10", got[0].Hash)
	require.Equal(t, "20", got[0].NonMetaHash)
	require.Equal(t, []model.LabelPair{
		{Name: "__address__", Value: "example.com:9090"},
		{Name: "__param_token", Value: "<redacted>"},
		{Name: "job", Value: "api"},
	}, got[0].Labels)
	require.NotNil(t, got[0].Scrape)
	require.Equal(t, "https://example.com:9090/metrics", got[0].Scrape.URL)
	require.Equal(t, model.ScrapeTargetHealthUp, got[0].Scrape.Health)
	require.Equal(t, lastAttempt, *got[0].Scrape.LastAttempt)
	require.Equal(t, model.Duration(250*time.Millisecond), *got[0].Scrape.LastDuration)
	require.Nil(t, got[0].Scrape.LastError)
}

func TestScrapeTargetWithoutAttemptHasNoAttemptDetails(t *testing.T) {
	info := targetInfoWithScrape(component.ScrapeTargetInfo{
		URL:    "https://example.com/metrics",
		Health: component.ScrapeTargetHealthUnknown,
	})

	got := targets.FromComponent(info)

	require.Len(t, got, 1)
	require.Equal(t, model.ScrapeTargetHealthUnknown, got[0].Scrape.Health)
	require.Nil(t, got[0].Scrape.LastAttempt)
	require.Nil(t, got[0].Scrape.LastDuration)
	require.Nil(t, got[0].Scrape.LastError)
}

func TestScrapeTargetRedactsURLAndError(t *testing.T) {
	const rawURL = "https://user:password@example.com/metrics?token=secret#fragment"
	lastAttempt := time.Date(2026, time.October, 1, 12, 30, 0, 0, time.UTC)
	info := targetInfoWithScrape(component.ScrapeTargetInfo{
		URL:          rawURL,
		Health:       component.ScrapeTargetHealthDown,
		LastAttempt:  lastAttempt,
		LastDuration: time.Second,
		LastError: &url.Error{
			Op:  "Get",
			URL: rawURL,
			Err: context.DeadlineExceeded,
		},
	})

	got := targets.FromComponent(info)

	require.Len(t, got, 1)
	require.Equal(t, "https://example.com/metrics", got[0].Scrape.URL)
	require.NotContains(t, *got[0].Scrape.LastError, "password")
	require.NotContains(t, *got[0].Scrape.LastError, "secret")
}

func TestScrapeTargetBoundsRuntimeErrorMessage(t *testing.T) {
	info := targetInfoWithScrape(component.ScrapeTargetInfo{
		URL:         "https://example.com/metrics",
		Health:      component.ScrapeTargetHealthDown,
		LastAttempt: time.Now(),
		LastError:   errors.New(strings.Repeat("x", 2048)),
	})

	got := targets.FromComponent(info)

	require.LessOrEqual(t, len(*got[0].Scrape.LastError), 1024)
}

func TestTargetLabelsExcludeMetaLabelsAndHashesDiffer(t *testing.T) {
	info := &component.Info{Exports: directTargetExports{
		Targets: []discovery.Target{discovery.NewTargetFromMap(map[string]string{
			"job":                       "api",
			"__meta_kubernetes_pod":     "api-0",
			"__address__":               "localhost:9090",
			"__meta_kubernetes_node":    "node-1",
			"__param_module":            "default",
			"__scrape_interval__":       "15s",
			"__meta_kubernetes_cluster": "prod",
		})},
	}}

	got := targets.FromComponent(info)

	require.Len(t, got, 1)
	require.Contains(t, got[0].Labels, model.LabelPair{Name: "__meta_kubernetes_pod", Value: "api-0"})
	require.NotContains(t, got[0].NonMetaLabels, model.LabelPair{Name: "__meta_kubernetes_pod", Value: "api-0"})
	require.Contains(t, got[0].NonMetaLabels, model.LabelPair{Name: "job", Value: "api"})
	require.NotEmpty(t, got[0].Hash)
	require.NotEmpty(t, got[0].NonMetaHash)
	require.NotEqual(t, got[0].Hash, got[0].NonMetaHash)
}

type directTargetExports struct {
	Targets []discovery.Target
}

type nestedTargetExports struct {
	Nested nestedTargets
}

type nestedTargets struct {
	Targets []discovery.Target
}

func targetInfoWithScrape(scrape component.ScrapeTargetInfo) *component.Info {
	return &component.Info{
		Component: &targetProvider{targets: []component.TargetInfo{{
			Scrape: &scrape,
		}}},
	}
}

type targetProvider struct {
	targets []component.TargetInfo
}

func (p *targetProvider) Run(context.Context) error {
	return nil
}

func (p *targetProvider) Update(component.Arguments) error {
	return nil
}

func (p *targetProvider) ComponentTargets() []component.TargetInfo {
	return p.targets
}
