package otelcolconvert

import (
	"fmt"
	"reflect"
	"strings"
	"unsafe"

	"github.com/grafana/alloy/internal/component/otelcol"
	"github.com/grafana/alloy/internal/component/otelcol/auth"
	"github.com/grafana/alloy/internal/component/otelcol/receiver/github"
	"github.com/grafana/alloy/internal/converter/diag"
	"github.com/grafana/alloy/internal/converter/internal/common"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/githubreceiver"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componentstatus"
	"go.opentelemetry.io/collector/pipeline"
)

func init() {
	converters = append(converters, githubReceiverConverter{})
}

type githubReceiverConverter struct{}

func (githubReceiverConverter) Factory() component.Factory {
	return githubreceiver.NewFactory()
}

func (githubReceiverConverter) InputComponentName() string { return "" }

func (githubReceiverConverter) ConvertAndAppend(state *State, id componentstatus.InstanceID, cfg component.Config) diag.Diagnostics {
	var diags diag.Diagnostics

	label := state.AlloyComponentLabel()

	args := toGithubReceiver(state, id, cfg.(*githubreceiver.Config))

	// Extract authenticator ID from scraper config before creating override hook
	// We can't import internal githubscraper package, so use reflection
	var authenticatorID component.ID
	scrapers := cfg.(*githubreceiver.Config).Scrapers
	if len(scrapers) > 0 {
		for _, sc := range scrapers {
			scVal := reflect.ValueOf(sc).Elem()
			if clientConfigField := scVal.FieldByName("ClientConfig"); clientConfigField.IsValid() {
				if authField := clientConfigField.FieldByName("Auth"); authField.IsValid() {
					// Auth has unexported "value" field - access via unsafe
					authPtr := unsafe.Pointer(authField.UnsafeAddr())
					authStruct := reflect.NewAt(authField.Type(), authPtr).Elem()

					if valueField := authStruct.FieldByName("value"); valueField.IsValid() {
						valuePtr := unsafe.Pointer(valueField.UnsafeAddr())
						valueStruct := reflect.NewAt(valueField.Type(), valuePtr).Elem()

						if idField := valueStruct.FieldByName("AuthenticatorID"); idField.IsValid() {
							idPtr := unsafe.Pointer(idField.UnsafeAddr())
							idValue := reflect.NewAt(idField.Type(), idPtr).Elem()
							authenticatorID = idValue.Interface().(component.ID)
						}
					}
				}
			}
			break
		}
	}

	// The override hook converts auth.Handler placeholders into component references
	overrideHook := func(val interface{}) interface{} {
		switch val.(type) {
		case *auth.Handler:
			if authenticatorID != (component.ID{}) {
				ext := state.LookupExtension(authenticatorID)
				return common.CustomTokenizer{Expr: fmt.Sprintf("%s.%s.handler", strings.Join(ext.Name, "."), ext.Label)}
			}
		}
		return val
	}

	block := common.NewBlockWithOverrideFn([]string{"otelcol", "receiver", "github"}, label, args, overrideHook)

	diags.Add(
		diag.SeverityLevelInfo,
		fmt.Sprintf("Converted %s into %s", StringifyInstanceID(id), StringifyBlock(block)),
	)

	state.Body().AppendBlock(block)
	return diags
}

func getScraperMap(cfg *githubreceiver.Config) map[string]any {
	for _, sc := range cfg.Scrapers {
		return encodeMapstruct(sc)
	}
	return nil
}

func toGithubReceiver(state *State, id componentstatus.InstanceID, cfg *githubreceiver.Config) *github.Arguments {
	nextMetrics := state.Next(id, pipeline.SignalMetrics)
	nextTraces := state.Next(id, pipeline.SignalTraces)

	var scraperConfig *github.ScraperConfig
	scraperMap := getScraperMap(cfg)
	if scraperMap != nil {
		scraperConfig = toScraperConfig(scraperMap)
	}

	return &github.Arguments{
		InitialDelay:       cfg.InitialDelay,
		CollectionInterval: cfg.CollectionInterval,
		Scraper:            scraperConfig,
		Webhook:            toWebhookConfig(cfg),
		DebugMetrics:       common.DefaultValue[github.Arguments]().DebugMetrics,
		Output: &otelcol.ConsumerArguments{
			Metrics: ToTokenizedConsumers(nextMetrics),
			Traces:  ToTokenizedConsumers(nextTraces),
		},
	}
}

func toScraperConfig(scraperMap map[string]any) *github.ScraperConfig {

	// Check if auth is configured
	authConfig := github.AuthConfig{}
	if authMap, ok := scraperMap["auth"].(map[string]any); ok {
		if authenticatorID, ok := authMap["authenticator"].(string); ok && authenticatorID != "" {
			// Create a placeholder auth.Handler that will be replaced by the override hook
			authConfig.Authenticator = &auth.Handler{}
		}
	}

	return &github.ScraperConfig{
		GithubOrg:   scraperMap["github_org"].(string),
		SearchQuery: scraperMap["search_query"].(string),
		Endpoint:    scraperMap["endpoint"].(string),
		Auth:        authConfig,
		Metrics:     toMetricsConfig(scraperMap),
	}
}

func toMetricsConfig(scraperMap map[string]any) github.MetricsConfig {
	metricsConfig := github.MetricsConfig{}

	// Extract metrics configuration if present
	// Only populate metrics that are explicitly configured in the input
	if metricsData, ok := scraperMap["metrics"].(map[string]any); ok {
		metricsConfig = github.MetricsConfig{
			VCSChangeCount:          toMetricConfig(metricsData, "vcs.change.count"),
			VCSChangeDuration:       toMetricConfig(metricsData, "vcs.change.duration"),
			VCSChangeTimeToApproval: toMetricConfig(metricsData, "vcs.change.time_to_approval"),
			VCSChangeTimeToMerge:    toMetricConfig(metricsData, "vcs.change.time_to_merge"),
			VCSRefCount:             toMetricConfig(metricsData, "vcs.ref.count"),
			VCSRefLinesDelta:        toMetricConfig(metricsData, "vcs.ref.lines_delta"),
			VCSRefRevisionsDelta:    toMetricConfig(metricsData, "vcs.ref.revisions_delta"),
			VCSRefTime:              toMetricConfig(metricsData, "vcs.ref.time"),
			VCSRepositoryCount:      toMetricConfig(metricsData, "vcs.repository.count"),
			VCSContributorCount:     toMetricConfig(metricsData, "vcs.contributor.count"),
		}
	}

	return metricsConfig
}

func toMetricConfig(metricsData map[string]any, metricName string) github.MetricConfig {
	if metricData, ok := metricsData[metricName].(map[string]any); ok {
		if enabled, ok := metricData["enabled"].(bool); ok {
			return github.MetricConfig{Enabled: enabled}
		}
	}

	if metricName == "vcs.contributor.count" {
		// Contributor count is disabled by default
		return github.MetricConfig{Enabled: false}
	}

	// Default to enabled if not specified
	return github.MetricConfig{Enabled: true}
}

func toWebhookConfig(cfg *githubreceiver.Config) *github.WebhookConfig {
	requiredHeaders := make(map[string]string)
	for k, v := range cfg.WebHook.RequiredHeaders {
		requiredHeaders[k] = string(v)
	}

	return &github.WebhookConfig{
		Endpoint:          cfg.WebHook.Endpoint,
		Path:              cfg.WebHook.Path,
		HealthPath:        cfg.WebHook.HealthPath,
		Secret:            cfg.WebHook.Secret,
		RequiredHeaders:   requiredHeaders,
		ServiceName:       cfg.WebHook.ServiceName,
		IncludeSpanEvents: cfg.WebHook.IncludeSpanEvents,
	}
}
