package config

import (
	"fmt"
	"reflect"

	v1 "github.com/grafana/alloy/internal/static/integrations"
	v2 "github.com/grafana/alloy/internal/static/integrations/v2"
	"github.com/grafana/alloy/internal/static/metrics"
	"github.com/grafana/alloy/internal/static/server"
	"github.com/grafana/alloy/internal/util"
	"gopkg.in/yaml.v2"
)

type IntegrationsVersion int

const (
	IntegrationsVersion1 IntegrationsVersion = iota
	IntegrationsVersion2
)

// DefaultVersionedIntegrations is the default config for integrations.
func DefaultVersionedIntegrations() VersionedIntegrations {
	configV1 := v1.DefaultManagerConfig()
	return VersionedIntegrations{
		Version:  IntegrationsVersion1,
		ConfigV1: &configV1,
	}
}

// VersionedIntegrations abstracts the subsystem configs for integrations v1
// and v2. VersionedIntegrations can only be unmarshaled as part of Load.
type VersionedIntegrations struct {
	Version IntegrationsVersion
	raw     util.RawYAML

	ConfigV1 *v1.ManagerConfig
	ConfigV2 *v2.SubsystemOptions

	// ExtraIntegrations is used when adding any integrations NOT in the default agent configuration
	ExtraIntegrations []v2.Config
}

var (
	_ yaml.Unmarshaler = (*VersionedIntegrations)(nil)
	_ yaml.Marshaler   = (*VersionedIntegrations)(nil)
)

// UnmarshalYAML implements yaml.Unmarshaler. Full unmarshaling is deferred until
// setVersion is invoked.
func (c *VersionedIntegrations) UnmarshalYAML(unmarshal func(any) error) error {
	c.ConfigV1 = nil
	c.ConfigV2 = nil
	return unmarshal(&c.raw)
}

// MarshalYAML implements yaml.Marshaler.
func (c VersionedIntegrations) MarshalYAML() (any, error) {
	switch {
	case c.ConfigV1 != nil:
		return c.ConfigV1, nil
	case c.ConfigV2 != nil:
		return c.ConfigV2, nil
	default:
		// A pointer is needed for the yaml.Marshaler implementation to work.
		return &c.raw, nil
	}
}

// IsZero implements yaml.IsZeroer.
func (c VersionedIntegrations) IsZero() bool {
	switch {
	case c.ConfigV1 != nil:
		return reflect.ValueOf(*c.ConfigV1).IsZero()
	case c.ConfigV2 != nil:
		return reflect.ValueOf(*c.ConfigV2).IsZero()
	default:
		return len(c.raw) == 0
	}
}

// ApplyDefaults applies defaults to the subsystem based on globals.
func (c *VersionedIntegrations) ApplyDefaults(sflags *server.Flags, mcfg *metrics.Config) error {
	switch {
	case c.Version != IntegrationsVersion2 && c.ConfigV1 != nil:
		return c.ConfigV1.ApplyDefaults(sflags, mcfg)
	case c.ConfigV2 != nil:
		return c.ConfigV2.ApplyDefaults(mcfg)
	default:
		// No-op
		return nil
	}
}

// setVersion completes the deferred unmarshal and unmarshals the raw YAML into
// the subsystem config for version v.
func (c *VersionedIntegrations) setVersion(v IntegrationsVersion) error {
	c.Version = v

	switch c.Version {
	case IntegrationsVersion1:
		// Do not overwrite the config if it's already been set. This is relevant for
		// cases where the config has already been loaded via other means (example: Agent
		// Management snippets).
		if c.ConfigV1 != nil {
			return nil
		}

		cfg := v1.DefaultManagerConfig()
		c.ConfigV1 = &cfg
		return yaml.UnmarshalStrict(c.raw, c.ConfigV1)
	case IntegrationsVersion2:
		cfg := v2.DefaultSubsystemOptions
		// this is needed for dynamic configuration, the unmarshal doesn't work correctly if
		// this is not nil.
		c.ConfigV1 = nil
		c.ConfigV2 = &cfg
		err := yaml.UnmarshalStrict(c.raw, c.ConfigV2)
		if err != nil {
			return err
		}
		c.ConfigV2.Configs = append(c.ConfigV2.Configs, c.ExtraIntegrations...)
		return nil
	default:
		panic(fmt.Sprintf("unknown integrations version %d", c.Version))
	}
}
