// Package solace provides an otelcol.receiver.solace component.
package solace

/*
How to test solace manually:
- Use the docker compose template here: https://github.com/SolaceLabs/solace-single-docker-compose/blob/master/template/PubSubStandard_singleNode.yml
- Log in to http://localhost:8080/ to configure solace and select default
- Go to Telemetry, enable it, create a profile "testprofile" and enable both trace and receiver
- Telemetry > Receiver connect ACLs > Client Connect Default Action: set it to "Allow" via the edit button
- Telemetry > Trace Filters: create a trace filter "testfilter" and enable it
- Click on the test filter, go to Subscriptions and create a new Subscription ">"
- In Queues, create a new queue "testqueue"
- Click on "testqueue" and create a subscription "solace/tracing"
- Access Control > Client Authentication: set the Type to "Internal database"
- Access Control > Client Username: create two clients:
  - "alloy", with all toggle enabled, the password set to "alloy" and the client profile and acl profile set to #telemetry-testprofile
  - "bob", with all toggle and the password set to "bob". Keep the client profile and acl profile set to default
- Connect Alloy with the following config:
	otelcol.receiver.solace "solace" {
		queue = "queue://#telemetry-testprofile"
		broker = "localhost:5672"
		auth {
			sasl_plain {
				username = "alloy"
				password = "alloy"
			}
		}
		tls {
			insecure             = true
			insecure_skip_verify = true
		}
		output {
			traces = [otelcol.exporter.debug.solace.input]
		}
	}

	otelcol.exporter.debug "solace" {
		verbosity = "detailed"
	}

	logging {
		level = "debug"
	}
- In "Try Me!", connect as "bob" (username and password)
- You should be able to see the span in the terminal
*/

import (
	"fmt"
	"strings"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/otelcol"
	otelcolCfg "github.com/grafana/alloy/internal/component/otelcol/config"
	"github.com/grafana/alloy/internal/component/otelcol/receiver"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/solacereceiver"
	otelcomponent "go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pipeline"
)

func init() {
	component.Register(component.Registration{
		Name:      "otelcol.receiver.solace",
		Stability: featuregate.StabilityGenerallyAvailable,
		Args:      Arguments{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			fact := solacereceiver.NewFactory()
			return receiver.New(opts, fact, args.(Arguments))
		},
	})
}

// Arguments configures the otelcol.receiver.solace component.
type Arguments struct {
	// The upstream component uses a list for the broker but they only use the first element in the list so I decided to use
	// a simple string in Alloy to avoid confusing the users.
	Broker     string `alloy:"broker,attr,optional"`
	Queue      string `alloy:"queue,attr"`
	MaxUnacked int32  `alloy:"max_unacknowledged,attr,optional"`

	TLS          otelcol.TLSClientArguments       `alloy:"tls,block,optional"`
	Flow         FlowControl                      `alloy:"flow_control,block,optional"`
	DebugMetrics otelcolCfg.DebugMetricsArguments `alloy:"debug_metrics,block,optional"`
	Auth         Authentication                   `alloy:"auth,block"`

	// Output configures where to send received data. Required.
	Output *otelcol.ConsumerArguments `alloy:"output,block"`
}

var _ receiver.Arguments = Arguments{}

// SetToDefault implements syntax.Defaulter.
func (args *Arguments) SetToDefault() {
	*args = Arguments{
		Broker:     "localhost:5671",
		MaxUnacked: 1000,
	}
	args.Flow.SetToDefault()
	args.DebugMetrics.SetToDefault()
}

// Validate implements syntax.Validator.
func (args *Arguments) Validate() error {
	authMethod := 0
	if args.Auth.PlainText != nil {
		authMethod++
	}
	if args.Auth.External != nil {
		authMethod++
	}
	if args.Auth.XAuth2 != nil {
		authMethod++
	}
	if authMethod != 1 {
		return fmt.Errorf("the auth block must contain exactly one of sasl_plain block, sasl_xauth2 block or sasl_external block")
	}
	if len(strings.TrimSpace(args.Queue)) == 0 {
		return fmt.Errorf("queue must not be empty, queue definition has format queue://<queuename>")
	}
	if args.Flow.DelayedRetry != nil && args.Flow.DelayedRetry.Delay <= 0 {
		return fmt.Errorf("the delay attribute in the delayed_retry block must be > 0, got %d", args.Flow.DelayedRetry.Delay)
	}
	return nil
}

// Convert implements receiver.Arguments.
func (args Arguments) Convert() (otelcomponent.Config, error) {
	return &solacereceiver.Config{
		Broker:     []string{args.Broker},
		Queue:      args.Queue,
		MaxUnacked: args.MaxUnacked,
		TLS:        *args.TLS.Convert(),
		Auth:       args.Auth.Convert(),
		Flow:       args.Flow.Convert(),
	}, nil
}

// Extensions implements receiver.Arguments.
func (args Arguments) Extensions() map[otelcomponent.ID]otelcomponent.Component {
	return nil
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
