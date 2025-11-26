package solace

import (
	"time"

	"github.com/grafana/alloy/syntax/alloytypes"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/solacereceiver"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/config/configoptional"
)

// Authentication defines authentication strategies.
type Authentication struct {
	PlainText *SaslPlainTextConfig `alloy:"sasl_plain,block,optional"`
	XAuth2    *SaslXAuth2Config    `alloy:"sasl_xauth2,block,optional"`
	External  *SaslExternalConfig  `alloy:"sasl_external,block,optional"`
}

// Convert converts args into the upstream type.
func (args Authentication) Convert() solacereceiver.Authentication {
	auth := solacereceiver.Authentication{}

	auth.PlainText = args.PlainText.Convert()
	auth.XAuth2 = args.XAuth2.Convert()
	auth.External = args.External.Convert()

	return auth
}

// SaslPlainTextConfig defines SASL PLAIN authentication.
type SaslPlainTextConfig struct {
	Username string            `alloy:"username,attr"`
	Password alloytypes.Secret `alloy:"password,attr"`
}

func (args *SaslPlainTextConfig) Convert() configoptional.Optional[solacereceiver.SaslPlainTextConfig] {
	if args == nil {
		return configoptional.None[solacereceiver.SaslPlainTextConfig]()
	}

	return configoptional.Some(solacereceiver.SaslPlainTextConfig{
		Username: args.Username,
		Password: configopaque.String(args.Password),
	})
}

// SaslXAuth2Config defines the configuration for the SASL XAUTH2 authentication.
type SaslXAuth2Config struct {
	Username string `alloy:"username,attr"`
	Bearer   string `alloy:"bearer,attr"`
}

func (args *SaslXAuth2Config) Convert() configoptional.Optional[solacereceiver.SaslXAuth2Config] {
	if args == nil {
		return configoptional.None[solacereceiver.SaslXAuth2Config]()
	}

	return configoptional.Some(solacereceiver.SaslXAuth2Config{
		Username: args.Username,
		Bearer:   args.Bearer,
	})
}

// SaslExternalConfig defines the configuration for the SASL External used in conjunction with TLS client authentication.
type SaslExternalConfig struct{}

func (args *SaslExternalConfig) Convert() configoptional.Optional[solacereceiver.SaslExternalConfig] {
	if args == nil {
		return configoptional.None[solacereceiver.SaslExternalConfig]()
	}

	return configoptional.Some(solacereceiver.SaslExternalConfig{})
}

// FlowControl defines the configuration for what to do in backpressure scenarios, e.g. memorylimiter errors
type FlowControl struct {
	DelayedRetry *FlowControlDelayedRetry `alloy:"delayed_retry,block"`
}

func (args FlowControl) Convert() solacereceiver.FlowControl {
	flowControl := solacereceiver.FlowControl{}
	flowControl.DelayedRetry = args.DelayedRetry.Convert()
	return flowControl
}

func (args *FlowControl) SetToDefault() {
	*args = FlowControl{
		DelayedRetry: &FlowControlDelayedRetry{
			Delay: 10 * time.Millisecond,
		},
	}
}

// FlowControlDelayedRetry represents the strategy of waiting for a defined amount of time (in time.Duration) and attempt redelivery
type FlowControlDelayedRetry struct {
	Delay time.Duration `alloy:"delay,attr,optional"`
}

func (args *FlowControlDelayedRetry) Convert() configoptional.Optional[solacereceiver.FlowControlDelayedRetry] {
	if args == nil {
		return configoptional.None[solacereceiver.FlowControlDelayedRetry]()
	}

	return configoptional.Some(solacereceiver.FlowControlDelayedRetry{
		Delay: args.Delay,
	})
}
