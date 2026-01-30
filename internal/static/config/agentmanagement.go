package config

import (
	"errors"
	"fmt"
	"time"

	"github.com/prometheus/common/config"
)

var (
	defaultRemoteConfiguration = RemoteConfiguration{
		AcceptHTTPNotModified: true,
	}
)

type labelMap map[string]string

type RemoteConfiguration struct {
	Labels                 labelMap `yaml:"labels"`
	LabelManagementEnabled bool     `yaml:"label_management_enabled"`
	AcceptHTTPNotModified  bool     `yaml:"accept_http_not_modified"`
	AgentID                string   `yaml:"agent_id"`
	Namespace              string   `yaml:"namespace"`
	CacheLocation          string   `yaml:"cache_location"`
}

// UnmarshalYAML implement YAML Unmarshaler
func (rc *RemoteConfiguration) UnmarshalYAML(unmarshal func(any) error) error {
	// Apply defaults
	*rc = defaultRemoteConfiguration
	type plain RemoteConfiguration
	return unmarshal((*plain)(rc))
}

type AgentManagementConfig struct {
	Enabled          bool                    `yaml:"-"` // Derived from enable-features=agent-management
	Host             string                  `yaml:"host"`
	Protocol         string                  `yaml:"protocol"`
	PollingInterval  time.Duration           `yaml:"polling_interval"`
	HTTPClientConfig config.HTTPClientConfig `yaml:",inline"`

	RemoteConfiguration RemoteConfiguration `yaml:"remote_configuration"`
}

// Validate checks that necessary portions of the config have been set.
func (am *AgentManagementConfig) Validate() error {
	if am.HTTPClientConfig.BasicAuth == nil || am.HTTPClientConfig.BasicAuth.Username == "" || am.HTTPClientConfig.BasicAuth.PasswordFile == "" {
		return errors.New("both username and password_file fields must be specified")
	}

	if am.PollingInterval <= 0 {
		return fmt.Errorf("polling interval must be >0")
	}

	if am.RemoteConfiguration.Namespace == "" {
		return errors.New("namespace must be specified in 'remote_configuration' block of the config")
	}

	if am.RemoteConfiguration.CacheLocation == "" {
		return errors.New("path to cache must be specified in 'agent_management.remote_configuration.cache_location'")
	}

	if am.RemoteConfiguration.LabelManagementEnabled && am.RemoteConfiguration.AgentID == "" {
		return errors.New("agent_id must be specified in 'agent_management.remote_configuration' if label_management_enabled is true")
	}

	return nil
}
