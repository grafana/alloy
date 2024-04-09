package config

import (
	"testing"
	"time"

	"github.com/prometheus/common/config"
	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v2"
)

var validAgentManagementConfig = AgentManagementConfig{
	Enabled: true,
	Host:    "localhost:1234",
	HTTPClientConfig: config.HTTPClientConfig{
		BasicAuth: &config.BasicAuth{
			Username:     "test",
			PasswordFile: "/test/path",
		},
	},

	Protocol:        "https",
	PollingInterval: time.Minute,
	RemoteConfiguration: RemoteConfiguration{
		Labels:        labelMap{"b": "B", "a": "A"},
		Namespace:     "test_namespace",
		CacheLocation: "/test/path/",
	},
}

func TestUnmarshalDefault(t *testing.T) {
	cfg := `host: "localhost:1234"
protocol: "https"
polling_interval: "1m"
remote_configuration:
  namespace: "test_namespace"`
	var am AgentManagementConfig
	err := yaml.Unmarshal([]byte(cfg), &am)
	assert.NoError(t, err)
	assert.True(t, am.RemoteConfiguration.AcceptHTTPNotModified)
	assert.Equal(t, "https", am.Protocol)
	assert.Equal(t, time.Minute, am.PollingInterval)
	assert.Equal(t, "test_namespace", am.RemoteConfiguration.Namespace)
}

func TestValidateValidConfig(t *testing.T) {
	assert.NoError(t, validAgentManagementConfig.Validate())
}

func TestValidateInvalidBasicAuth(t *testing.T) {
	invalidConfig := &AgentManagementConfig{
		Enabled:          true,
		Host:             "localhost:1234",
		HTTPClientConfig: config.HTTPClientConfig{},
		Protocol:         "https",
		PollingInterval:  time.Minute,
		RemoteConfiguration: RemoteConfiguration{
			Namespace:     "test_namespace",
			CacheLocation: "/test/path/",
		},
	}
	// This should error as the BasicAuth is nil
	assert.Error(t, invalidConfig.Validate())

	// This should error as the BasicAuth is empty
	invalidConfig.HTTPClientConfig.BasicAuth = &config.BasicAuth{}
	assert.Error(t, invalidConfig.Validate())

	invalidConfig.HTTPClientConfig.BasicAuth.Username = "test"
	assert.Error(t, invalidConfig.Validate()) // Should still error as there is no password file set

	invalidConfig.HTTPClientConfig.BasicAuth.Username = ""
	invalidConfig.HTTPClientConfig.BasicAuth.PasswordFile = "/test/path"
	assert.Error(t, invalidConfig.Validate()) // Should still error as there is no username set
}

func TestMissingCacheLocation(t *testing.T) {
	invalidConfig := &AgentManagementConfig{
		Enabled: true,
		Host:    "localhost:1234",
		HTTPClientConfig: config.HTTPClientConfig{
			BasicAuth: &config.BasicAuth{
				Username:     "test",
				PasswordFile: "/test/path",
			},
		},
		Protocol:        "https",
		PollingInterval: 1 * time.Minute,
		RemoteConfiguration: RemoteConfiguration{
			Namespace: "test_namespace",
		},
	}
	assert.Error(t, invalidConfig.Validate())
}

func TestValidateLabelManagement(t *testing.T) {
	cfg := &AgentManagementConfig{
		Enabled: true,
		Host:    "localhost:1234",
		HTTPClientConfig: config.HTTPClientConfig{
			BasicAuth: &config.BasicAuth{
				Username:     "test",
				PasswordFile: "/test/path",
			},
		},
		Protocol:        "https",
		PollingInterval: time.Minute,
		RemoteConfiguration: RemoteConfiguration{
			Namespace:              "test_namespace",
			CacheLocation:          "/test/path/",
			LabelManagementEnabled: true,
		},
	}
	// This should error as there is no agent_id set
	assert.Error(t, cfg.Validate())

	cfg.RemoteConfiguration.AgentID = "test_agent_id"
	assert.NoError(t, cfg.Validate())
}
