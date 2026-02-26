// Package testhelper provides shared test data and helpers for the secretfilter
// component tests. It is used by both the secretfilter package tests and the
// extend subpackage tests so that config-loading tests run in separate processes
// without duplicating test data or run logic.
package testhelper

import (
	"fmt"

	"github.com/grafana/alloy/internal/service/livedebugging"
)

// FakeSecret holds a fake secret value for tests.
type FakeSecret struct {
	Name  string
	Value string
}

// TestLog holds a log line and optional metadata for tests.
type TestLog struct {
	Log string
}

// FakeSecrets is the shared map of fake secrets used to build test log lines.
var FakeSecrets = map[string]FakeSecret{
	"grafana-api-key": {
		Name:  "grafana-api-key",
		Value: "eyJr" + "IjoiT0x6NWJuNDRuZkI1NHJ6dEJrR0g3aDVuRnY0NWJuNDRuZkI1NHJ6dEJrR0g3aDVuRnY0NWJuNDRuZkI1NHJ6dEJrR0g3aDVuRnY0NWJuNDRuZkI1NHJ6dEJrR0g3aDVu",
	},
	"gcp-api-key": {
		Name:  "gcp-api-key",
		Value: "AIza" + "SyDaGmWKa4JsXZ-HjGw7ISLn_3namBGewQe",
	},
	"stripe-key": {
		Name:  "stripe-access-token",
		Value: "sk_live_" + "51HFxYz2eZvKYlo2C9kKM5nE6qO4yKn8N3bP7hXxYz2eZvKYlo2C",
	},
	"npm-token": {
		Name:  "npm-access-token",
		Value: "npm_" + "1A2b3C4d5E6f7G8h9I0jK1lM2nO3pQ4rS5tU",
	},
	"generic-api-key": {
		Name:  "generic-api-key",
		Value: "token:" + "Aa1Bb2Cc3Dd4Ee5Ff6Gg7Hh8Ii9Jj0Kk",
	},
}

// TestLogs is the shared map of test log entries. Built in init so it can reference FakeSecrets.
var TestLogs map[string]TestLog

func init() {
	TestLogs = map[string]TestLog{
		"no_secret": {
			Log: `{
			"message": "This is a simple log message"
		}`,
		},
		"grafana_api_key": {
			Log: `{
			"message": "This is a simple log message with a secret value ` + FakeSecrets["grafana-api-key"].Value + ` !
		}`,
		},
		"gcp_api_key": {
			Log: `{
			"message": "This is a simple log message with a secret value ` + FakeSecrets["gcp-api-key"].Value + ` !
		}`,
		},
		"stripe_key": {
			Log: `{
			"message": "This is a simple log message with a secret value ` + FakeSecrets["stripe-key"].Value + ` !
		}`,
		},
		"npm_token": {
			Log: `{
			"message": "This is a simple log message with a secret value ` + FakeSecrets["npm-token"].Value + ` !
		}`,
		},
		"generic_api_key": {
			Log: `{
			"message": "This is a simple log message with a secret value ` + FakeSecrets["generic-api-key"].Value + ` !
		}`,
		},
		"multiple_secrets": {
			Log: `{
			"message": "This is a simple log message with a secret value ` + FakeSecrets["grafana-api-key"].Value + ` and another secret value ` + FakeSecrets["gcp-api-key"].Value + ` !
		}`,
		},
	}
}

// TestConfigs holds shared Alloy config snippets for tests.
var TestConfigs = map[string]string{
	"default": `
		forward_to = []
	`,
	"with_origin": `
		forward_to = []
		origin_label = "job"
	`,
	"with_redact_percent_100": `
		forward_to = []
		redact_percent = 100
	`,
	"with_redact_percent_80": `
		forward_to = []
		redact_percent = 80
	`,
	"with_redact_with": `
		forward_to = []
		redact_with = "***REDACTED***"
	`,
}

// Case is a single test case: name, log line, and whether redaction is expected.
type Case struct {
	Name         string
	InputLog     string
	ShouldRedact bool
}

// DefaultCases are the 7 standard cases (no_secret through multiple_secrets) for default config.
// Set in init() so TestLogs is populated first.
var DefaultCases []Case

func init() {
	DefaultCases = []Case{
		{"no_secret", TestLogs["no_secret"].Log, false},
		{"grafana_api_key", TestLogs["grafana_api_key"].Log, true},
		{"gcp_api_key", TestLogs["gcp_api_key"].Log, true},
		{"stripe_key", TestLogs["stripe_key"].Log, true},
		{"npm_token", TestLogs["npm_token"].Log, true},
		{"generic_api_key", TestLogs["generic_api_key"].Log, true},
		{"multiple_secrets", TestLogs["multiple_secrets"].Log, true},
	}
}

// GetServiceData returns the livedebugging service for component tests.
func GetServiceData(name string) (any, error) {
	switch name {
	case livedebugging.ServiceName:
		return livedebugging.NewLiveDebugging(), nil
	default:
		return nil, fmt.Errorf("service not found %s", name)
	}
}
