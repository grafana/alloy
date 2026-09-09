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
	Config ConfigSchema    `yaml:"config"`
	Inputs InputSchema     `yaml:"inputs"`
	Assert AssertionSchema `yaml:"assert"`
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
}

// AssertionSchema groups declarative assertions by signal type.
type AssertionSchema struct {
	Loki       []LokiAssertionSchema       `yaml:"loki"`
	Prometheus []PrometheusAssertionSchema `yaml:"prometheus"`
}

// MapMatchSchema describes map-like fields such as labels or structured
// metadata. Mode controls whether Values must match exactly or be contained in
// the actual field.
type MapMatchSchema struct {
	Mode   string            `yaml:"mode,omitempty"`
	Values map[string]string `yaml:"values,omitempty"`
}

// produceInputs sends all configured test inputs into the running pipeline.
func produceInputs(ctx context.Context, alloy *harness.Alloy, inputs InputSchema) error {
	return produceLokiInputs(ctx, alloy, inputs.Loki)
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

	return append(lokiAssertions, promAssertions...), nil
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
