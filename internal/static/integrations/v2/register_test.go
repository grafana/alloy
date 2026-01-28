package integrations

import (
	"testing"
	"time"

	"github.com/go-kit/log"
	v1 "github.com/grafana/alloy/internal/static/integrations"
	"github.com/grafana/alloy/internal/static/integrations/v2/common"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

func TestIntegrationRegistration(t *testing.T) {
	setRegistered(t, map[Config]Type{
		&testIntegrationA{}: TypeEither,
		&testIntegrationB{}: TypeEither,
	})

	// This test checks for a few things:
	//
	// 1. Registered integrations will be parseable
	// 2. Registered integrations that are not present will not be unmarshaled to
	//    the list of configs
	// 3. Registered integrations that have defaults may still be parsed
	// 4. Strict parsing should still work as expected.

	var cfgToParse = `
name: John Doe
duration: 500ms
test:
  text: Hello, world!
`

	var fullCfg testFullConfig
	err := yaml.UnmarshalStrict([]byte(cfgToParse), &fullCfg)
	require.NoError(t, err)

	expect := testFullConfig{
		Name:     "John Doe",
		Duration: 500 * time.Millisecond,
		Default:  12345,
		Configs: []Config{
			&testIntegrationA{Text: "Hello, world!", Truth: true},
		},
	}
	require.Equal(t, expect, fullCfg)
}

func TestIntegrationRegistration_Multiple(t *testing.T) {
	setRegistered(t, map[Config]Type{
		&testIntegrationA{}: TypeEither,
		&testIntegrationB{}: TypeEither,
	})

	var cfgToParse = `
name: John Doe
duration: 500ms
test_configs:
  - text: Hello, world!
  - text: Hello again!`

	var fullCfg testFullConfig
	err := yaml.UnmarshalStrict([]byte(cfgToParse), &fullCfg)
	require.NoError(t, err)

	expect := testFullConfig{
		Name:     "John Doe",
		Duration: 500 * time.Millisecond,
		Default:  12345,
		Configs: []Config{
			&testIntegrationA{Text: "Hello, world!", Truth: true},
			&testIntegrationA{Text: "Hello again!", Truth: true},
		},
	}
	require.Equal(t, expect, fullCfg)
}

func TestIntegrationRegistration_Mixed(t *testing.T) {
	setRegistered(t, map[Config]Type{
		&testIntegrationA{}: TypeEither,
		&testIntegrationB{}: TypeEither,
	})

	var cfgToParse = `
name: John Doe
duration: 500ms
test:
  text: Hello, world!
test_configs:
  - text: Hello again!`

	var fullCfg testFullConfig
	err := yaml.UnmarshalStrict([]byte(cfgToParse), &fullCfg)
	require.NoError(t, err)

	expect := testFullConfig{
		Name:     "John Doe",
		Duration: 500 * time.Millisecond,
		Default:  12345,
		Configs: []Config{
			&testIntegrationA{Text: "Hello, world!", Truth: true},
			&testIntegrationA{Text: "Hello again!", Truth: true},
		},
	}
	require.Equal(t, expect, fullCfg)
}

func TestIntegrationRegistration_Legacy(t *testing.T) {
	setRegistered(t, nil)

	RegisterLegacy(&legacyConfig{}, TypeSingleton, func(in v1.Config, mc common.MetricsConfig) UpgradedConfig {
		return &legacyShim{Data: in, Common: mc}
	})

	var cfgToParse = `
name: John Doe
duration: 500ms
legacy:
  text: hello`

	var fullCfg testFullConfig
	err := yaml.UnmarshalStrict([]byte(cfgToParse), &fullCfg)
	require.NoError(t, err)

	require.Len(t, fullCfg.Configs, 1)
	require.IsType(t, &legacyShim{}, fullCfg.Configs[0])

	shim := fullCfg.Configs[0].(*legacyShim)
	require.IsType(t, &legacyConfig{}, shim.Data)

	v1Config := shim.Data.(*legacyConfig)
	require.Equal(t, "hello", v1Config.Text)
}

func TestIntegrationRegistration_Legacy_Multiplex(t *testing.T) {
	setRegistered(t, nil)

	RegisterLegacy(&legacyConfig{}, TypeMultiplex, func(in v1.Config, mc common.MetricsConfig) UpgradedConfig {
		return &legacyShim{Data: in, Common: mc}
	})

	var cfgToParse = `
name: John Doe
duration: 500ms
legacy_configs:
  - text: hello
  - text: world`

	var fullCfg testFullConfig
	err := yaml.UnmarshalStrict([]byte(cfgToParse), &fullCfg)
	require.NoError(t, err)

	require.Len(t, fullCfg.Configs, 2)
	require.IsType(t, &legacyShim{}, fullCfg.Configs[0])
	require.IsType(t, &legacyShim{}, fullCfg.Configs[1])

	shim := fullCfg.Configs[0].(*legacyShim)
	require.IsType(t, &legacyConfig{}, shim.Data)
	require.Equal(t, "hello", shim.Data.(*legacyConfig).Text)

	shim = fullCfg.Configs[1].(*legacyShim)
	require.IsType(t, &legacyConfig{}, shim.Data)
	require.Equal(t, "world", shim.Data.(*legacyConfig).Text)
}

func TestIntegrationRegistration_Marshal_MultipleSingleton(t *testing.T) {
	setRegistered(t, map[Config]Type{
		&testIntegrationA{}: TypeSingleton,
		&testIntegrationB{}: TypeSingleton,
	})

	// Generate an invalid config, which has two instances of a Singleton
	// integration.
	input := testFullConfig{
		Name:     "John Doe",
		Duration: 500 * time.Millisecond,
		Default:  12345,
		Configs: []Config{
			&testIntegrationA{Text: "Hello, world!", Truth: true},
			&testIntegrationA{Text: "Hello again!", Truth: true},
		},
	}

	_, err := yaml.Marshal(&input)
	require.EqualError(t, err, `integration "test" may not be defined more than once`)
}

func TestIntegrationRegistration_Marshal_Multiplex(t *testing.T) {
	setRegistered(t, map[Config]Type{
		&testIntegrationA{}: TypeMultiplex,
		&testIntegrationB{}: TypeMultiplex,
	})

	// Generate an invalid config, which has two instances of a Singleton
	// integration.
	input := testFullConfig{
		Name:     "John Doe",
		Duration: 500 * time.Millisecond,
		Default:  12345,
		Configs: []Config{
			&testIntegrationA{Text: "Hello, world!", Truth: true},
			&testIntegrationA{Text: "Hello again!", Truth: true},
		},
	}

	expectedCfg := `name: John Doe
duration: 500ms
default: 12345
test_configs:
- text: Hello, world!
  truth: true
- text: Hello again!
  truth: true
`

	cfg, err := yaml.Marshal(&input)
	require.NoError(t, err)
	require.Equal(t, expectedCfg, string(cfg))
}

type legacyConfig struct {
	Text string `yaml:"text"`
}

func (lc *legacyConfig) Name() string                                        { return "legacy" }
func (lc *legacyConfig) InstanceKey(agentKey string) (string, error)         { return agentKey, nil }
func (lc *legacyConfig) NewIntegration(l log.Logger) (v1.Integration, error) { return nil, nil }

type legacyShim struct {
	Data   v1.Config
	Common common.MetricsConfig
}

func (s *legacyShim) LegacyConfig() (v1.Config, common.MetricsConfig) { return s.Data, s.Common }
func (s *legacyShim) Name() string                                    { return s.Data.Name() }
func (s *legacyShim) ApplyDefaults(g Globals) error {
	s.Common.ApplyDefaults(g.SubsystemOpts.Metrics.Autoscrape)
	return nil
}
func (s *legacyShim) Identifier(g Globals) (string, error) { return g.AgentIdentifier, nil }
func (s *legacyShim) NewIntegration(log.Logger, Globals) (Integration, error) {
	return NoOpIntegration, nil
}

type testIntegrationA struct {
	Text  string `yaml:"text"`
	Truth bool   `yaml:"truth"`
}

func (i *testIntegrationA) Name() string                       { return "test" }
func (i *testIntegrationA) ApplyDefaults(Globals) error        { return nil }
func (i *testIntegrationA) Identifier(Globals) (string, error) { return "integrationA", nil }
func (i *testIntegrationA) NewIntegration(log.Logger, Globals) (Integration, error) {
	return NoOpIntegration, nil
}

func (i *testIntegrationA) UnmarshalYAML(unmarshal func(any) error) error {
	i.Truth = true
	type plain testIntegrationA
	return unmarshal((*plain)(i))
}

type testIntegrationB struct {
	Text string `yaml:"text"`
}

func (*testIntegrationB) Name() string                       { return "shouldnotbefound" }
func (*testIntegrationB) ApplyDefaults(Globals) error        { return nil }
func (*testIntegrationB) Identifier(Globals) (string, error) { return "integrationB", nil }
func (*testIntegrationB) NewIntegration(log.Logger, Globals) (Integration, error) {
	return NoOpIntegration, nil
}

type testFullConfig struct {
	// Some random fields that will also be exposed
	Name     string        `yaml:"name"`
	Duration time.Duration `yaml:"duration"`
	Default  int           `yaml:"default"`

	Configs Configs `yaml:"-"`
}

func (c *testFullConfig) UnmarshalYAML(unmarshal func(any) error) error {
	// This default value should not change.
	c.Default = 12345
	return UnmarshalYAML(c, unmarshal)
}

func (c testFullConfig) MarshalYAML() (any, error) {
	return MarshalYAML(c)
}
