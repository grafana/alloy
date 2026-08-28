// Stanza is the name of the logs agent that was donated to the OpenTelemetry project.
// Stanza receivers are logs receivers built of stanza operators.
package otelcol

import (
	"errors"
	"time"

	"github.com/grafana/alloy/syntax"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/split"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/trim"
	"go.opentelemetry.io/collector/confmap"
)

var (
	_ syntax.Defaulter = (*ConsumerRetryArguments)(nil)
)

// ConsumerRetryArguments holds shared settings for stanza receivers which can retry
// requests. There is no Convert functionality as the consumerretry package is stanza internal
type ConsumerRetryArguments struct {
	Enabled         bool          `alloy:"enabled,attr,optional"`
	InitialInterval time.Duration `alloy:"initial_interval,attr,optional"`
	MaxInterval     time.Duration `alloy:"max_interval,attr,optional"`
	MaxElapsedTime  time.Duration `alloy:"max_elapsed_time,attr,optional"`
}

// SetToDefault implements syntax.Defaulter.
func (args *ConsumerRetryArguments) SetToDefault() {
	*args = ConsumerRetryArguments{
		Enabled:         false,
		InitialInterval: 1 * time.Second,
		MaxInterval:     30 * time.Second,
		MaxElapsedTime:  5 * time.Minute,
	}
}

type TrimConfig struct {
	PreserveLeadingWhitespace  bool `alloy:"preserve_leading_whitespaces,attr,optional"`
	PreserveTrailingWhitespace bool `alloy:"preserve_trailing_whitespaces,attr,optional"`
}

func (c *TrimConfig) Convert() *trim.Config {
	if c == nil {
		return nil
	}

	return &trim.Config{
		PreserveLeading:  c.PreserveLeadingWhitespace,
		PreserveTrailing: c.PreserveTrailingWhitespace,
	}
}

type MultilineConfig struct {
	LineStartPattern string `alloy:"line_start_pattern,attr,optional"`
	LineEndPattern   string `alloy:"line_end_pattern,attr,optional"`
	OmitPattern      bool   `alloy:"omit_pattern,attr,optional"`
}

func (c *MultilineConfig) Convert() *split.Config {
	if c == nil {
		return nil
	}

	return &split.Config{
		LineStartPattern: c.LineStartPattern,
		LineEndPattern:   c.LineEndPattern,
		OmitPattern:      c.OmitPattern,
	}
}

func (c *MultilineConfig) Validate() error {
	if c == nil {
		return nil
	}

	if c.LineStartPattern == "" && c.LineEndPattern == "" {
		return errors.New("either line_start_pattern or line_end_pattern must be set")
	}

	if c.LineStartPattern != "" && c.LineEndPattern != "" {
		return errors.New("only one of line_start_pattern or line_end_pattern can be set")
	}

	return nil
}

type Operator map[string]any

func (o Operator) Convert() (*operator.Config, error) {
	cfg := &operator.Config{}
	err := cfg.Unmarshal(confmap.NewFromStringMap(o))
	return cfg, err
}
