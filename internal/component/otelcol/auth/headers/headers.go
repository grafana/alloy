// Package headers provides an otelcol.auth.headers component.
package headers

import (
	"encoding"
	"fmt"
	"strings"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/otelcol/auth"
	otelcolCfg "github.com/grafana/alloy/internal/component/otelcol/config"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/syntax"
	"github.com/grafana/alloy/syntax/alloytypes"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/headerssetterextension"
	otelcomponent "go.opentelemetry.io/collector/component"
	otelextension "go.opentelemetry.io/collector/extension"
)

func init() {
	component.Register(component.Registration{
		Name:      "otelcol.auth.headers",
		Stability: featuregate.StabilityGenerallyAvailable,
		Args:      Arguments{},
		Exports:   auth.Exports{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			fact := headerssetterextension.NewFactory()
			return auth.New(opts, fact, args.(Arguments))
		},
	})
}

// Arguments configures the otelcol.auth.headers component.
type Arguments struct {
	Headers []Header `alloy:"header,block,optional"`

	// DebugMetrics configures component internal metrics. Optional.
	DebugMetrics otelcolCfg.DebugMetricsArguments `alloy:"debug_metrics,block,optional"`
}

var _ auth.Arguments = Arguments{}

// SetToDefault implements syntax.Defaulter.
func (args *Arguments) SetToDefault() {
	args.DebugMetrics.SetToDefault()
}

// Convert implements auth.Arguments.
func (args Arguments) Convert() (otelcomponent.Config, error) {
	var upstreamHeaders []headerssetterextension.HeaderConfig
	for _, h := range args.Headers {
		upstreamHeader := headerssetterextension.HeaderConfig{
			Key: &h.Key,
		}

		err := h.Action.Convert(&upstreamHeader)
		if err != nil {
			return nil, err
		}

		if h.Value != nil {
			upstreamHeader.Value = &h.Value.Value
		}
		if h.FromContext != nil {
			upstreamHeader.FromContext = h.FromContext
		}

		upstreamHeaders = append(upstreamHeaders, upstreamHeader)
	}

	return &headerssetterextension.Config{
		HeadersConfig: upstreamHeaders,
	}, nil
}

// Extensions implements auth.Arguments.
func (args Arguments) Extensions() map[otelcomponent.ID]otelextension.Extension {
	return nil
}

// Exporters implements auth.Arguments.
func (args Arguments) Exporters() map[otelcomponent.DataType]map[otelcomponent.ID]otelcomponent.Component {
	return nil
}

// DebugMetricsConfig implements auth.Arguments.
func (args Arguments) DebugMetricsConfig() otelcolCfg.DebugMetricsArguments {
	return args.DebugMetrics
}

type Action string

const (
	ActionInsert Action = "insert"
	ActionUpdate Action = "update"
	ActionUpsert Action = "upsert"
	ActionDelete Action = "delete"
)

var (
	_ syntax.Validator         = (*Action)(nil)
	_ encoding.TextUnmarshaler = (*Action)(nil)
)

// Validate implements syntax.Validator.
func (a *Action) Validate() error {
	switch *a {
	case ActionInsert, ActionUpdate, ActionUpsert, ActionDelete:
		// This is a valid value, do not error
	default:
		return fmt.Errorf("action is set to an invalid value of %q", *a)
	}
	return nil
}

// Convert the Alloy type to the Otel type.
// TODO: When headerssetterextension.actionValue is made external,
// remove the input parameter and make this output the Otel type.
func (a *Action) Convert(hc *headerssetterextension.HeaderConfig) error {
	switch *a {
	case ActionInsert:
		hc.Action = headerssetterextension.INSERT
	case ActionUpdate:
		hc.Action = headerssetterextension.UPDATE
	case ActionUpsert:
		hc.Action = headerssetterextension.UPSERT
	case ActionDelete:
		hc.Action = headerssetterextension.DELETE
	default:
		return fmt.Errorf("action is set to an invalid value of %q", *a)
	}
	return nil
}

func (a *Action) UnmarshalText(text []byte) error {
	str := Action(strings.ToLower(string(text)))
	switch str {
	case ActionInsert, ActionUpdate, ActionUpsert, ActionDelete:
		*a = str
		return nil
	default:
		return fmt.Errorf("unknown action %v", str)
	}
}

// Header is an individual Header to send along with requests.
type Header struct {
	Key         string                     `alloy:"key,attr"`
	Value       *alloytypes.OptionalSecret `alloy:"value,attr,optional"`
	FromContext *string                    `alloy:"from_context,attr,optional"`
	Action      Action                     `alloy:"action,attr,optional"`
}

var _ syntax.Defaulter = &Header{}

var DefaultHeader = Header{
	Action: ActionUpsert,
}

// SetToDefault implements syntax.Defaulter.
func (h *Header) SetToDefault() {
	*h = DefaultHeader
}

// Validate implements syntax.Validator.
func (h *Header) Validate() error {
	err := h.Action.Validate()
	if err != nil {
		return err
	}

	switch {
	case h.Key == "":
		return fmt.Errorf("key must be set to a non-empty string")
	case h.FromContext == nil && h.Value == nil:
		return fmt.Errorf("either value or from_context must be provided")
	case h.FromContext != nil && h.Value != nil:
		return fmt.Errorf("either value or from_context must be provided, not both")
	}

	return nil
}
