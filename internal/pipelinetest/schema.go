package pipelinetest

import (
	"context"
	"errors"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"

	"github.com/grafana/alloy/internal/pipelinetest/harness"
)

const (
	mapMatchModeExact   = "exact"
	mapMatchModePartial = "partial"
)

// TestSchema describes a declarative pipeline test loaded from a text file.
type TestSchema struct {
	// OS limits the test to one operating system, using a runtime.GOOS value
	// such as "windows". The test is skipped on other systems.
	OS string `yaml:"os,omitempty"`
	// Timeout is how long to wait for the assertions to pass, as a Go
	// duration. It defaults to 30 seconds.
	Timeout string `yaml:"timeout,omitempty"`
	// Setup prepares files and runs commands before Alloy starts.
	Setup  SetupSchema     `yaml:"setup,omitempty"`
	Flags  FlagsSchema     `yaml:"flags,omitempty"`
	Config ConfigSchema    `yaml:"config"`
	Inputs InputSchema     `yaml:"inputs"`
	Mocks  MockSchema      `yaml:"mocks"`
	Assert AssertionSchema `yaml:"assert"`
}

// FlagsSchema holds the Alloy run flags enabled for a pipeline test.
type FlagsSchema struct {
	LokiPipelineV2 bool `yaml:"feature.loki.pipeline-v2.enabled,omitempty"`
}

type ConfigSchema string

func (c *ConfigSchema) UnmarshalYAML(value *yaml.Node) error {
	switch value.Kind {
	case yaml.ScalarNode:
		*c = ConfigSchema(value.Value)
		return nil
	case yaml.MappingNode:
		var cfg struct {
			Path string `yaml:"path"`
		}
		if err := value.Decode(&cfg); err != nil {
			return err
		}
		if cfg.Path == "" {
			return errors.New("config mapping requires path")
		}

		bb, err := os.ReadFile(cfg.Path)
		if err != nil {
			return fmt.Errorf("read config path %q: %w", cfg.Path, err)
		}

		*c = ConfigSchema(bb)
		return nil
	default:
		return fmt.Errorf("config must be inline or from a path")
	}
}

// InputSchema groups pipeline test inputs by signal type.
type InputSchema struct {
	Loki []LokiInputSchema `yaml:"loki"`
	HTTP []HTTPInputSchema `yaml:"http"`
	TCP  []TCPInputSchema  `yaml:"tcp"`
	UDP  []UDPInputSchema  `yaml:"udp"`
}

// AssertionSchema groups declarative assertions by signal type.
type AssertionSchema struct {
	Loki       []LokiAssertionSchema       `yaml:"loki"`
	Prometheus []PrometheusAssertionSchema `yaml:"prometheus"`
	Logs       []LogAssertionSchema        `yaml:"logs"`
}

// LogAssertionSchema describes one assertion about the log output of Alloy.
type LogAssertionSchema struct {
	// Contains is text that some log line must contain.
	Contains string `yaml:"contains"`
}

// MapMatchSchema describes map-like fields such as labels or structured
// metadata. Mode controls whether Values must match exactly or be contained in
// the actual field.
type MapMatchSchema struct {
	Mode   string            `yaml:"mode,omitempty"`
	Values map[string]string `yaml:"values,omitempty"`
	// Regex maps a label name to an anchored regular expression that its
	// value must match. It only applies to Prometheus labels.
	Regex map[string]string `yaml:"regex,omitempty"`
}

// produceInputs sends all configured test inputs into the running pipeline,
// in this order: Loki entries, HTTP requests, TCP data, then UDP datagrams.
func produceInputs(ctx context.Context, alloy *harness.Alloy, inputs InputSchema) error {
	if err := produceLokiInputs(ctx, alloy, inputs.Loki); err != nil {
		return err
	}
	if err := produceHTTPInputs(ctx, inputs.HTTP); err != nil {
		return err
	}
	if err := produceTCPInputs(ctx, inputs.TCP); err != nil {
		return err
	}
	return produceUDPInputs(ctx, alloy, inputs.UDP)
}

func buildAssertions(assertions AssertionSchema) ([]harness.Assertion, error) {
	lokiAssertions, err := buildLokiAssertions(assertions.Loki)
	if err != nil {
		return nil, err
	}

	promAssertions, err := buildPrometheusAssertions(assertions.Prometheus)
	if err != nil {
		return nil, err
	}

	out := append(lokiAssertions, promAssertions...)
	for _, assertion := range assertions.Logs {
		if assertion.Contains == "" {
			return nil, errors.New("logs assertion requires contains")
		}
		out = append(out, harness.LogsContain(assertion.Contains))
	}
	return out, nil
}

func isPartialMapMatch(name string, match MapMatchSchema) (bool, error) {
	switch match.Mode {
	case "", mapMatchModeExact:
		return false, nil
	case mapMatchModePartial:
		return true, nil
	default:
		return false, fmt.Errorf("%s mode must be %q or %q, got %q", name, mapMatchModeExact, mapMatchModePartial, match.Mode)
	}
}
