package kafka

import (
	"context"
	"sync"

	"github.com/IBM/sarama"
	"github.com/grafana/agent/internal/component"
	"github.com/grafana/agent/internal/component/common/config"
	"github.com/grafana/agent/internal/component/common/loki"
	flow_relabel "github.com/grafana/agent/internal/component/common/relabel"
	kt "github.com/grafana/agent/internal/component/loki/source/internal/kafkatarget"
	"github.com/grafana/agent/internal/featuregate"
	"github.com/grafana/agent/internal/flow/logging/level"
	"github.com/grafana/dskit/flagext"
	"github.com/grafana/river/rivertypes"
	"github.com/prometheus/common/model"
)

func init() {
	component.Register(component.Registration{
		Name:      "loki.source.kafka",
		Stability: featuregate.StabilityStable,
		Args:      Arguments{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return New(opts, args.(Arguments))
		},
	})
}

// Arguments holds values which are used to configure the loki.source.kafka
// component.
type Arguments struct {
	Brokers              []string            `river:"brokers,attr"`
	Topics               []string            `river:"topics,attr"`
	GroupID              string              `river:"group_id,attr,optional"`
	Assignor             string              `river:"assignor,attr,optional"`
	Version              string              `river:"version,attr,optional"`
	Authentication       KafkaAuthentication `river:"authentication,block,optional"`
	UseIncomingTimestamp bool                `river:"use_incoming_timestamp,attr,optional"`
	Labels               map[string]string   `river:"labels,attr,optional"`

	ForwardTo    []loki.LogsReceiver `river:"forward_to,attr"`
	RelabelRules flow_relabel.Rules  `river:"relabel_rules,attr,optional"`
}

// KafkaAuthentication describe the configuration for authentication with Kafka brokers
type KafkaAuthentication struct {
	Type       string           `river:"type,attr,optional"`
	TLSConfig  config.TLSConfig `river:"tls_config,block,optional"`
	SASLConfig KafkaSASLConfig  `river:"sasl_config,block,optional"`
}

// KafkaSASLConfig describe the SASL configuration for authentication with Kafka brokers
type KafkaSASLConfig struct {
	Mechanism   string            `river:"mechanism,attr,optional"`
	User        string            `river:"user,attr,optional"`
	Password    rivertypes.Secret `river:"password,attr,optional"`
	UseTLS      bool              `river:"use_tls,attr,optional"`
	TLSConfig   config.TLSConfig  `river:"tls_config,block,optional"`
	OAuthConfig OAuthConfigConfig `river:"oauth_config,block,optional"`
}

type OAuthConfigConfig struct {
	TokenProvider string   `river:"token_provider,attr"`
	Scopes        []string `river:"scopes,attr"`
}

// DefaultArguments provides the default arguments for a kafka component.
var DefaultArguments = Arguments{
	GroupID:  "loki.source.kafka",
	Assignor: "range",
	Version:  "2.2.1",
	Authentication: KafkaAuthentication{
		Type: "none",
		SASLConfig: KafkaSASLConfig{
			Mechanism: sarama.SASLTypePlaintext,
			UseTLS:    false,
		},
	},
	UseIncomingTimestamp: false,
}

// SetToDefault implements river.Defaulter.
func (a *Arguments) SetToDefault() {
	*a = DefaultArguments
}

// Component implements the loki.source.kafka component.
type Component struct {
	opts component.Options

	mut    sync.RWMutex
	fanout []loki.LogsReceiver
	target *kt.TargetSyncer

	handler loki.LogsReceiver
}

// New creates a new loki.source.kafka component.
func New(o component.Options, args Arguments) (*Component, error) {
	c := &Component{
		opts:    o,
		mut:     sync.RWMutex{},
		fanout:  args.ForwardTo,
		target:  nil,
		handler: loki.NewLogsReceiver(),
	}

	// Call to Update() to start readers and set receivers once at the start.
	if err := c.Update(args); err != nil {
		return nil, err
	}

	return c, nil
}

// Run implements component.Component.
func (c *Component) Run(ctx context.Context) error {
	defer func() {
		c.mut.Lock()
		defer c.mut.Unlock()

		level.Info(c.opts.Logger).Log("msg", "loki.source.kafka component shutting down, stopping target")
		if c.target != nil {
			err := c.target.Stop()
			if err != nil {
				level.Error(c.opts.Logger).Log("msg", "error while stopping kafka target", "err", err)
			}
		}
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

// Update implements component.Component.
func (c *Component) Update(args component.Arguments) error {
	c.mut.Lock()
	defer c.mut.Unlock()

	newArgs := args.(Arguments)
	c.fanout = newArgs.ForwardTo

	if c.target != nil {
		err := c.target.Stop()
		if err != nil {
			level.Error(c.opts.Logger).Log("msg", "error while stopping kafka target", "err", err)
		}
	}

	entryHandler := loki.NewEntryHandler(c.handler.Chan(), func() {})
	t, err := kt.NewSyncer(c.opts.Logger, newArgs.Convert(), entryHandler, &kt.KafkaTargetMessageParser{})
	if err != nil {
		level.Error(c.opts.Logger).Log("msg", "failed to create kafka client with provided config", "err", err)
		return err
	}

	c.target = t

	return nil
}

// Convert is used to bridge between the River and Promtail types.
func (args *Arguments) Convert() kt.Config {
	lbls := make(model.LabelSet, len(args.Labels))
	for k, v := range args.Labels {
		lbls[model.LabelName(k)] = model.LabelValue(v)
	}

	return kt.Config{
		KafkaConfig: kt.TargetConfig{
			Labels:               lbls,
			UseIncomingTimestamp: args.UseIncomingTimestamp,
			Brokers:              args.Brokers,
			GroupID:              args.GroupID,
			Topics:               args.Topics,
			Version:              args.Version,
			Assignor:             args.Assignor,
			Authentication:       args.Authentication.Convert(),
		},
		RelabelConfigs: flow_relabel.ComponentToPromRelabelConfigs(args.RelabelRules),
	}
}

func (auth KafkaAuthentication) Convert() kt.Authentication {
	return kt.Authentication{
		Type:      kt.AuthenticationType(auth.Type),
		TLSConfig: *auth.TLSConfig.Convert(),
		SASLConfig: kt.SASLConfig{
			Mechanism: sarama.SASLMechanism(auth.SASLConfig.Mechanism),
			User:      auth.SASLConfig.User,
			Password:  flagext.SecretWithValue(string(auth.SASLConfig.Password)),
			UseTLS:    auth.SASLConfig.UseTLS,
			TLSConfig: *auth.SASLConfig.TLSConfig.Convert(),
			OAuthConfig: kt.OAuthConfig{
				TokenProvider: kt.TokenProviderType(auth.SASLConfig.OAuthConfig.TokenProvider),
				Scopes:        auth.SASLConfig.OAuthConfig.Scopes,
			},
		},
	}
}
