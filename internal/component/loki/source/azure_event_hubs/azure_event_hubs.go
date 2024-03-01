package azure_event_hubs

import (
	"context"
	"fmt"
	"net"
	"sync"

	"github.com/IBM/sarama"
	"github.com/grafana/agent/internal/component"
	"github.com/grafana/agent/internal/component/common/loki"
	flow_relabel "github.com/grafana/agent/internal/component/common/relabel"
	"github.com/grafana/agent/internal/component/loki/source/azure_event_hubs/internal/parser"
	kt "github.com/grafana/agent/internal/component/loki/source/internal/kafkatarget"
	"github.com/grafana/agent/internal/featuregate"
	"github.com/grafana/agent/internal/flow/logging/level"
	"github.com/grafana/dskit/flagext"

	"github.com/prometheus/common/model"
)

func init() {
	component.Register(component.Registration{
		Name:      "loki.source.azure_event_hubs",
		Stability: featuregate.StabilityStable,
		Args:      Arguments{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return New(opts, args.(Arguments))
		},
	})
}

// Arguments holds values which are used to configure the loki.source.azure_event_hubs component.
type Arguments struct {
	FullyQualifiedNamespace string   `river:"fully_qualified_namespace,attr"`
	EventHubs               []string `river:"event_hubs,attr"`

	Authentication AzureEventHubsAuthentication `river:"authentication,block"`

	GroupID                string             `river:"group_id,attr,optional"`
	UseIncomingTimestamp   bool               `river:"use_incoming_timestamp,attr,optional"`
	DisallowCustomMessages bool               `river:"disallow_custom_messages,attr,optional"`
	RelabelRules           flow_relabel.Rules `river:"relabel_rules,attr,optional"`
	Labels                 map[string]string  `river:"labels,attr,optional"`
	Assignor               string             `river:"assignor,attr,optional"`

	ForwardTo []loki.LogsReceiver `river:"forward_to,attr"`
}

// AzureEventHubsAuthentication describe the configuration for authentication with Azure Event Hub
type AzureEventHubsAuthentication struct {
	Mechanism        string   `river:"mechanism,attr"`
	Scopes           []string `river:"scopes,attr,optional"`
	ConnectionString string   `river:"connection_string,attr,optional"`
}

func getDefault() Arguments {
	return Arguments{
		GroupID:  "loki.source.azure_event_hubs",
		Labels:   map[string]string{"job": "loki.source.azure_event_hubs"},
		Assignor: "range",
	}
}

// SetToDefault implements river.Defaulter.
func (a *Arguments) SetToDefault() {
	*a = getDefault()
}

// Validate implements river.Validator.
func (a *Arguments) Validate() error {
	return a.validateAssignor()
}

// New creates a new loki.source.azure_event_hubs component.
func New(o component.Options, args Arguments) (*Component, error) {
	c := &Component{
		mut:     sync.RWMutex{},
		opts:    o,
		handler: loki.NewLogsReceiver(),
		fanout:  args.ForwardTo,
	}

	// Call to Update() to start readers and set receivers once at the start.
	if err := c.Update(args); err != nil {
		return nil, err
	}

	return c, nil
}

// Component implements the loki.source.azure_event_hubs component.
type Component struct {
	opts    component.Options
	mut     sync.RWMutex
	fanout  []loki.LogsReceiver
	handler loki.LogsReceiver
	target  *kt.TargetSyncer
}

// Run implements component.Component.
func (c *Component) Run(ctx context.Context) error {
	defer func() {
		level.Info(c.opts.Logger).Log("msg", "loki.source.azure_event_hubs component shutting down, stopping the targets")
		c.mut.RLock()
		err := c.target.Stop()
		if err != nil {
			level.Error(c.opts.Logger).Log("msg", "error while stopping azure_event_hubs target", "err", err)
		}
		c.mut.RUnlock()
	}()

	for {
		select {
		case <-ctx.Done():
			return nil
		case entry := <-c.handler.Chan():
			c.mut.RLock()
			for _, receiver := range c.fanout {
				receiver.Chan() <- entry
			}
			c.mut.RUnlock()
		}
	}
}

const (
	AuthenticationMechanismConnectionString = "connection_string"
	AuthenticationMechanismOAuth            = "oauth"
)

// Update implements component.Component.
func (c *Component) Update(args component.Arguments) error {
	c.mut.Lock()
	defer c.mut.Unlock()

	newArgs := args.(Arguments)
	c.fanout = newArgs.ForwardTo

	cfg, err := newArgs.Convert()
	if err != nil {
		return err
	}

	entryHandler := loki.NewEntryHandler(c.handler.Chan(), func() {})
	t, err := kt.NewSyncer(c.opts.Logger, cfg, entryHandler, &parser.AzureEventHubsTargetMessageParser{
		DisallowCustomMessages: newArgs.DisallowCustomMessages,
	})
	if err != nil {
		return fmt.Errorf("error starting azure_event_hubs target: %w", err)
	}
	c.target = t

	return nil
}

// Convert is used to bridge between the River and Promtail types.
func (a *Arguments) Convert() (kt.Config, error) {
	lbls := make(model.LabelSet, len(a.Labels))
	for k, v := range a.Labels {
		lbls[model.LabelName(k)] = model.LabelValue(v)
	}

	cfg := kt.Config{
		RelabelConfigs: flow_relabel.ComponentToPromRelabelConfigs(a.RelabelRules),
		KafkaConfig: kt.TargetConfig{
			Brokers:              []string{a.FullyQualifiedNamespace},
			Topics:               a.EventHubs,
			Labels:               lbls,
			UseIncomingTimestamp: a.UseIncomingTimestamp,
			GroupID:              a.GroupID,
			Version:              sarama.V1_0_0_0.String(),
			Assignor:             a.Assignor,
		},
	}
	switch a.Authentication.Mechanism {
	case AuthenticationMechanismConnectionString:
		if a.Authentication.ConnectionString == "" {
			return kt.Config{}, fmt.Errorf("connection string is required when authentication mechanism is %s", a.Authentication.Mechanism)
		}
		cfg.KafkaConfig.Authentication = kt.Authentication{
			Type: kt.AuthenticationTypeSASL,
			SASLConfig: kt.SASLConfig{
				UseTLS:    true,
				User:      "$ConnectionString",
				Password:  flagext.SecretWithValue(a.Authentication.ConnectionString),
				Mechanism: sarama.SASLTypePlaintext,
			},
		}
	case AuthenticationMechanismOAuth:
		if a.Authentication.Scopes == nil {
			host, _, err := net.SplitHostPort(a.FullyQualifiedNamespace)
			if err != nil {
				return kt.Config{}, fmt.Errorf("unable to extract host from fully qualified namespace: %w", err)
			}
			a.Authentication.Scopes = []string{fmt.Sprintf("https://%s/.default", host)}
		}

		cfg.KafkaConfig.Authentication = kt.Authentication{
			Type: kt.AuthenticationTypeSASL,
			SASLConfig: kt.SASLConfig{
				UseTLS:    true,
				Mechanism: sarama.SASLTypeOAuth,
				OAuthConfig: kt.OAuthConfig{
					TokenProvider: kt.TokenProviderTypeAzure,
					Scopes:        a.Authentication.Scopes,
				},
			},
		}
	default:
		return kt.Config{}, fmt.Errorf("authentication mechanism %s is unsupported", a.Authentication.Mechanism)
	}
	return cfg, nil
}

func (a *Arguments) validateAssignor() error {
	validAssignors := []string{sarama.StickyBalanceStrategyName, sarama.RoundRobinBalanceStrategyName, sarama.RangeBalanceStrategyName}
	for _, validAssignor := range validAssignors {
		if a.Assignor == validAssignor {
			return nil
		}
	}
	return fmt.Errorf("assignor value %s is invalid, must be one of: %v", a.Assignor, validAssignors)
}
