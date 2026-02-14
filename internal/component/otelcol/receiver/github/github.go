package github

import (
	"time"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/otelcol"
	"github.com/grafana/alloy/internal/component/otelcol/auth"
	otelcolCfg "github.com/grafana/alloy/internal/component/otelcol/config"
	"github.com/grafana/alloy/internal/component/otelcol/receiver"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/githubreceiver"
	otelcomponent "go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/pipeline"
)

func init() {
	component.Register(component.Registration{
		Name:      "otelcol.receiver.github",
		Stability: featuregate.StabilityExperimental,
		Args:      Arguments{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			fact := githubreceiver.NewFactory()
			return receiver.New(opts, fact, args.(Arguments))
		},
	})
}

// Arguments configures the otelcol.receiver.github component.
type Arguments struct {
	InitialDelay       time.Duration                    `alloy:"initial_delay,attr,optional"`
	CollectionInterval time.Duration                    `alloy:"collection_interval,attr,optional"`
	Scraper            *ScraperConfig                   `alloy:"scraper,block,optional"`
	Webhook            *WebhookConfig                   `alloy:"webhook,block,optional"`
	DebugMetrics       otelcolCfg.DebugMetricsArguments `alloy:"debug_metrics,block,optional"`

	// Output configures where to send received data. Required.
	Output *otelcol.ConsumerArguments `alloy:"output,block"`
}

var _ receiver.Arguments = Arguments{}

func (args *Arguments) Validate() error {
	if args.Scraper != nil {
		if err := args.Scraper.Validate(); err != nil {
			return err
		}
	}
	return nil
}

func (args *Arguments) SetToDefault() {
	if args.CollectionInterval == 0 {
		args.CollectionInterval = 30 * time.Second
	}

	if args.Scraper != nil {
		args.Scraper.SetToDefault()
	}

	if args.Webhook != nil {
		args.Webhook.SetToDefault()
	}

	args.DebugMetrics.SetToDefault()
}

func (args Arguments) Convert() (otelcomponent.Config, error) {
	// Build a map representation of the config
	configMap := map[string]interface{}{
		"initial_delay":       args.InitialDelay,
		"collection_interval": args.CollectionInterval,
	}

	if args.Scraper != nil {
		scraperMap, err := args.Scraper.Convert()
		if err != nil {
			return nil, err
		}
		configMap["scrapers"] = map[string]interface{}{
			"scraper": scraperMap,
		}
	}

	if args.Webhook != nil {
		configMap["webhook"] = args.Webhook.Convert()
	}

	// Create a confmap and use the receiver's Unmarshal method
	// This allows the receiver to properly initialize internal types
	conf := confmap.NewFromStringMap(configMap)
	config := &githubreceiver.Config{}

	err := config.Unmarshal(conf)
	if err != nil {
		return nil, err
	}

	return config, nil
}

// Extensions implements receiver.Arguments.
func (args Arguments) Extensions() map[otelcomponent.ID]otelcomponent.Component {
	m := make(map[otelcomponent.ID]otelcomponent.Component)

	// Register auth extension if scraper is configured
	if args.Scraper != nil && args.Scraper.Auth.Authenticator != nil {
		ext, err := args.Scraper.Auth.Authenticator.GetExtension(auth.Client)
		if err == nil {
			m[ext.ID] = ext.Extension
		}
	}

	return m
}

// Exporters implements receiver.Arguments.
func (args Arguments) Exporters() map[pipeline.Signal]map[otelcomponent.ID]otelcomponent.Component {
	return nil
}

// NextConsumers implements receiver.Arguments.
func (args Arguments) NextConsumers() *otelcol.ConsumerArguments {
	return args.Output
}

// DebugMetricsConfig implements receiver.Arguments.
func (args Arguments) DebugMetricsConfig() otelcolCfg.DebugMetricsArguments {
	return args.DebugMetrics
}
