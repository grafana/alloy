package github_test

import (
	"testing"
	"time"

	otelcolCfg "github.com/grafana/alloy/internal/component/otelcol/config"
	"github.com/grafana/alloy/internal/component/otelcol/receiver/github"
	"github.com/stretchr/testify/require"
)

func TestArguments_SetToDefault(t *testing.T) {
	args := github.Arguments{}
	args.SetToDefault()

	// Default collection_interval should be 30s
	require.Equal(t, 30*time.Second, args.CollectionInterval)
	require.Equal(t, time.Duration(0), args.InitialDelay)
}

func TestArguments_Validate(t *testing.T) {
	t.Run("valid config with scraper", func(t *testing.T) {
		args := github.Arguments{}
		args.SetToDefault()

		// Create a scraper config
		scraper := &github.ScraperConfig{
			GithubOrg:        "test-org",
			SearchQuery:      "is:pr",
			ConcurrencyLimit: 5,
		}
		args.Scraper = scraper

		// Should pass validation
		require.NoError(t, args.Validate())
	})

	t.Run("missing github_org", func(t *testing.T) {
		args := github.Arguments{}
		args.SetToDefault()

		// Create a scraper config without github_org
		scraper := &github.ScraperConfig{
			SearchQuery: "is:pr",
		}
		args.Scraper = scraper

		// Should fail validation
		err := args.Validate()
		require.Error(t, err)
		require.Contains(t, err.Error(), "github_org")
	})

	t.Run("no scraper configured", func(t *testing.T) {
		args := github.Arguments{}
		args.SetToDefault()

		// Should pass validation even with no scraper
		require.NoError(t, args.Validate())
	})
}

func TestDebugMetricsConfig(t *testing.T) {
	tests := []struct {
		testName string
		setup    func() github.Arguments
		expected otelcolCfg.DebugMetricsArguments
	}{
		{
			testName: "default",
			setup: func() github.Arguments {
				args := github.Arguments{}
				args.SetToDefault()
				return args
			},
			expected: otelcolCfg.DebugMetricsArguments{
				DisableHighCardinalityMetrics: true,
				Level:                         otelcolCfg.LevelDetailed,
			},
		},
		{
			testName: "explicit_false",
			setup: func() github.Arguments {
				args := github.Arguments{}
				args.SetToDefault()
				args.DebugMetrics.DisableHighCardinalityMetrics = false
				return args
			},
			expected: otelcolCfg.DebugMetricsArguments{
				DisableHighCardinalityMetrics: false,
				Level:                         otelcolCfg.LevelDetailed,
			},
		},
		{
			testName: "explicit_true",
			setup: func() github.Arguments {
				args := github.Arguments{}
				args.SetToDefault()
				args.DebugMetrics.DisableHighCardinalityMetrics = true
				return args
			},
			expected: otelcolCfg.DebugMetricsArguments{
				DisableHighCardinalityMetrics: true,
				Level:                         otelcolCfg.LevelDetailed,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.testName, func(t *testing.T) {
			args := tc.setup()
			require.Equal(t, tc.expected, args.DebugMetricsConfig())
		})
	}
}

func TestConfigDefault(t *testing.T) {
	args := github.Arguments{}
	args.SetToDefault()

	// Default config should have 30s collection interval
	require.Equal(t, 30*time.Second, args.CollectionInterval)

	// Should not error with no scraper configured
	require.NoError(t, args.Validate())
}

func TestScraperConfig_Validate(t *testing.T) {
	t.Run("valid config", func(t *testing.T) {
		scraper := github.ScraperConfig{
			GithubOrg:        "test-org",
			SearchQuery:      "is:pr",
			ConcurrencyLimit: 5,
		}
		require.NoError(t, scraper.Validate())
	})

	t.Run("missing github_org", func(t *testing.T) {
		scraper := github.ScraperConfig{
			SearchQuery: "is:pr",
		}
		err := scraper.Validate()
		require.Error(t, err)
		require.Contains(t, err.Error(), "github_org")
	})
}

func TestScraperConfig_SetToDefault(t *testing.T) {
	scraper := github.ScraperConfig{}
	scraper.SetToDefault()

	// Metrics should have defaults set
	require.NotNil(t, scraper.Metrics)
}

func TestWebhookConfig_SetToDefault(t *testing.T) {
	webhook := github.WebhookConfig{}
	webhook.SetToDefault()

	// Should set default values
	require.Equal(t, "localhost:8080", webhook.Endpoint)
	require.Equal(t, "/events", webhook.Path)
	require.Equal(t, "/health", webhook.HealthPath)
}

func TestMetricsConfig_SetToDefault(t *testing.T) {
	metrics := github.MetricsConfig{}
	metrics.SetToDefault()

	// Most metrics should be enabled by default
	require.True(t, metrics.VCSChangeCount.Enabled)
	require.True(t, metrics.VCSChangeDuration.Enabled)
	require.True(t, metrics.VCSRepositoryCount.Enabled)
	// Contributor count should be disabled by default
	require.False(t, metrics.VCSContributorCount.Enabled)
}
