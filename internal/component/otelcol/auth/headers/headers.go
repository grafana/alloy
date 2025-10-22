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
	"go.opentelemetry.io/collector/pipeline"
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

// ConvertClient implements auth.Arguments.
func (args Arguments) ConvertClient() (otelcomponent.Config, error) {
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

		if h.FromAttribute != nil {
			upstreamHeader.FromAttribute = h.FromAttribute
		}

		upstreamHeaders = append(upstreamHeaders, upstreamHeader)
	}

	// OtelExtensionConfig does not implement ServerAuth
	return &headerssetterextension.Config{
		HeadersConfig: upstreamHeaders,
	}, nil
}

// ConvertServer returns nil since theheaders extension does not support server authentication.
func (args Arguments) ConvertServer() (otelcomponent.Config, error) {
	return nil, nil
}

// Extensions implements auth.Arguments.
func (args Arguments) Extensions() map[otelcomponent.ID]otelcomponent.Component {
	return nil
}

// AuthFeatures implements auth.Arguments.
func (args Arguments) AuthFeatures() auth.AuthFeature {
	return auth.ClientAuthSupported
}

// Exporters implements auth.Arguments.
func (args Arguments) Exporters() map[pipeline.Signal]map[otelcomponent.ID]otelcomponent.Component {
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
	Key           string                     `alloy:"key,attr"`
	Value         *alloytypes.OptionalSecret `alloy:"value,attr,optional"`
	FromContext   *string                    `alloy:"from_context,attr,optional"`
	FromAttribute *string                    `alloy:"from_attribute,attr,optional"`
	Action        Action                     `alloy:"action,attr,optional"`
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

	sources := 0
	if h.Value != nil {
		sources++
	}
	if h.FromContext != nil {
		sources++
	}
	if h.FromAttribute != nil {
		sources++
	}

	switch {
	case h.Key == "":
		return fmt.Errorf("key must be set to a non-empty string")
	case sources == 0:
		return fmt.Errorf("one of value, from_context, or from_attribute must be provided")
	case sources > 1:
		return fmt.Errorf("only one of value, from_context, or from_attribute may be provided")
	}

	return nil
}
