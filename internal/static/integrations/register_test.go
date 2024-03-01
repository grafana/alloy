package integrations

import (
	"fmt"
	"testing"
	"time"

	"github.com/go-kit/log"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

func TestIntegrationRegistration(t *testing.T) {
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
		Configs: []UnmarshaledConfig{{
			Config: &testIntegrationA{Text: "Hello, world!", Truth: true},
		}},
	}
	require.Equal(t, expect, fullCfg)
}

type testIntegrationA struct {
	Text  string `yaml:"text"`
	Truth bool   `yaml:"truth"`
}

func (i *testIntegrationA) Name() string                         { return "test" }
func (i *testIntegrationA) InstanceKey(_ string) (string, error) { return "integrationA", nil }

func (i *testIntegrationA) NewIntegration(l log.Logger) (Integration, error) {
	return nil, fmt.Errorf("not implemented")
}

func (i *testIntegrationA) UnmarshalYAML(unmarshal func(interface{}) error) error {
	i.Truth = true
	type plain testIntegrationA
	return unmarshal((*plain)(i))
}

type testIntegrationB struct {
	Text string `yaml:"text"`
}

func (*testIntegrationB) Name() string                         { return "shouldnotbefound" }
func (*testIntegrationB) InstanceKey(_ string) (string, error) { return "integrationB", nil }

func (*testIntegrationB) NewIntegration(l log.Logger) (Integration, error) {
	return nil, fmt.Errorf("not implemented")
}

type testFullConfig struct {
	// Some random fields that will also be exposed
	Name     string        `yaml:"name"`
	Duration time.Duration `yaml:"duration"`
	Default  int           `yaml:"default"`

	Configs Configs `yaml:"-"`
}

func (c *testFullConfig) UnmarshalYAML(unmarshal func(interface{}) error) error {
	// This default value should not change.
	c.Default = 12345

	// Mock out registered integrations.
	registered := []Config{
		&testIntegrationA{},
		&testIntegrationB{},
	}
	return unmarshalIntegrationsWithList(registered, c, unmarshal)
}
