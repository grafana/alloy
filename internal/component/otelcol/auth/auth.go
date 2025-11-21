// Package auth provides utilities to create an Alloy component from
// OpenTelemetry Collector authentication extensions.
//
// Other OpenTelemetry Collector extensions are better served as generic Alloy
// components rather than being placed in the otelcol namespace.
package auth

import (
	"context"
	"errors"
	"fmt"
	"hash/fnv"
	"strings"

	"github.com/grafana/alloy/internal/component"
	otelcolCfg "github.com/grafana/alloy/internal/component/otelcol/config"
	"github.com/grafana/alloy/internal/component/otelcol/internal/lazycollector"
	"github.com/grafana/alloy/internal/component/otelcol/internal/scheduler"
	otelcolutil "github.com/grafana/alloy/internal/component/otelcol/util"
	"github.com/grafana/alloy/internal/util/zapadapter"
	"github.com/grafana/alloy/syntax"
	"github.com/prometheus/client_golang/prometheus"
	otelcomponent "go.opentelemetry.io/collector/component"
	otelextension "go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/collector/pipeline"
	sdkprometheus "go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/sdk/metric"
)

var (
	ErrNotServerExtension = errors.New("component does not support server authentication")
	ErrNotClientExtension = errors.New("component does not support client authentication")
)

type ExtensionType string
type AuthFeature byte

const (
	ClientAuthSupported          AuthFeature = 1 << iota
	ServerAuthSupported          AuthFeature = 1 << iota
	ClientAndServerAuthSupported AuthFeature = ClientAuthSupported | ServerAuthSupported

	Server ExtensionType = "server"
	Client ExtensionType = "client"
)

// Arguments is an extension of component.Arguments which contains necessary
// settings for OpenTelemetry Collector authentication extensions.
type Arguments interface {
	component.Arguments

	// AuthFeature returns the type of auth that a opentelemetry collector plugin supports
	// client auth, server auth or both.
	AuthFeatures() AuthFeature

	// ConvertClient converts the Arguments into an OpenTelemetry Collector
	// client authentication extension configuration. If the plugin does
	// not support server authentication it should return nil, nil
	ConvertClient() (otelcomponent.Config, error)

	// ConvetServer converts the Arguments into an OpenTelemetry Collector
	// server authentication extension configuration. If the plugin does
	// not support server authentication it should return nil, nil
	ConvertServer() (otelcomponent.Config, error)

	// Extensions returns the set of extensions that the configured component is
	// allowed to use.
	Extensions() map[otelcomponent.ID]otelcomponent.Component

	// Exporters returns the set of exporters that are exposed to the configured
	// component.
	Exporters() map[pipeline.Signal]map[otelcomponent.ID]otelcomponent.Component

	// DebugMetricsConfig returns the configuration for debug metrics
	DebugMetricsConfig() otelcolCfg.DebugMetricsArguments
}

// Exports is a common Exports type for Alloy components which expose
// OpenTelemetry Collector authentication extensions.
type Exports struct {
	// Handler is the managed component. Handler is updated any time the
	// extension is updated.
	Handler *Handler `alloy:"handler,attr"`
}

// Handler combines an extension with its ID.
type Handler struct {
	componentID string
	handlerMap  map[ExtensionType]*ExtensionHandler
}

// NewHandler creates a handler that can be exported
// in a capsule for otel servers to consume.
func NewHandler(componentID string) *Handler {
	return &Handler{
		componentID: componentID,
		handlerMap:  map[ExtensionType]*ExtensionHandler{},
	}
}

// GetExtension retrieves the extension for the requested auth type, server or client.
func (h *Handler) GetExtension(et ExtensionType) (*ExtensionHandler, error) {
	ext, ok := h.handlerMap[et]

	// This condition shouldn't happen since both extension types are set in Update(), but
	// this will prevent a panic if it is somehow unset.
	if !ok {
		return nil, fmt.Errorf("error getting %s auth extension. component %s was unexpectedly nil", et, h.componentID)
	}

	// Check to make sure the extension does not have Error set.
	// see SetupExtension() to see how this value is set.
	// In general the error value is set if an auth extension
	// does not support the type of authentication that was requested.
	if ext.Error != nil {
		return nil, ext.Error
	}

	return ext, nil
}

// AddExtension registers an extension type with the handler so it can be referenced
// by another component.
func (h *Handler) AddExtension(et ExtensionType, eh *ExtensionHandler) error {
	// If an invalid extension is passed raise an error.
	if et != Server && et != Client {
		return fmt.Errorf("invalid extension type %s", et)
	}

	if eh == nil {
		return fmt.Errorf("extension handler must not be null")
	}

	h.handlerMap[et] = eh
	return nil
}

type ExtensionHandler struct {
	ID        otelcomponent.ID
	Extension otelextension.Extension

	// Set if the extension does not support the type of authentication
	// requested
	Error error
}

var _ syntax.Capsule = Handler{}

// AlloyCapsule marks Handler as a capsule type.
func (Handler) AlloyCapsule() {}

// Auth is an Alloy component shim which manages an OpenTelemetry Collector
// authentication extension.
type Auth struct {
	ctx    context.Context
	cancel context.CancelFunc

	opts    component.Options
	factory otelextension.Factory

	sched     *scheduler.Scheduler
	collector *lazycollector.Collector
}

var (
	_ component.Component       = (*Auth)(nil)
	_ component.HealthComponent = (*Auth)(nil)
)

// New creates a new Alloy component which encapsulates an OpenTelemetry
// Collector authentication extension. args must hold a value of the argument
// type registered with the Alloy component.
//
// The registered component must be registered to export the Exports type from
// this package, otherwise New will panic.
func New(opts component.Options, f otelextension.Factory, args Arguments) (*Auth, error) {
	ctx, cancel := context.WithCancel(context.Background())

	// Create a lazy collector where metrics from the upstream component will be
	// forwarded.
	collector := lazycollector.New()
	opts.Registerer.MustRegister(collector)

	r := &Auth{
		ctx:    ctx,
		cancel: cancel,

		opts:    opts,
		factory: f,

		sched:     scheduler.New(opts.Logger),
		collector: collector,
	}
	if err := r.Update(args); err != nil {
		return nil, err
	}
	return r, nil
}

// Run starts the Auth component.
func (a *Auth) Run(ctx context.Context) error {
	defer a.cancel()
	return a.sched.Run(ctx)
}

// Update implements component.Component. It will convert the Arguments into
// configuration for OpenTelemetry Collector authentication extension
// configuration and manage the underlying OpenTelemetry Collector extension.
func (a *Auth) Update(args component.Arguments) error {
	rargs := args.(Arguments)

	host := scheduler.NewHost(
		a.opts.Logger,
		scheduler.WithHostExtensions(rargs.Extensions()),
		scheduler.WithHostExporters(rargs.Exporters()),
	)

	reg := prometheus.NewRegistry()
	a.collector.Set(reg)

	promExporter, err := sdkprometheus.New(sdkprometheus.WithRegisterer(reg), sdkprometheus.WithoutTargetInfo())
	if err != nil {
		return err
	}

	mp := metric.NewMeterProvider(metric.WithReader(promExporter))
	settings := otelextension.Settings{
		ID: otelcomponent.NewIDWithName(a.factory.Type(), a.opts.ID),
		TelemetrySettings: otelcomponent.TelemetrySettings{
			Logger:         zapadapter.New(a.opts.Logger),
			TracerProvider: a.opts.Tracer,
			MeterProvider:  mp,
		},

		BuildInfo: otelcolutil.GetBuildInfo(),
	}

	resource, err := otelcolutil.GetTelemetrySettingsResource()
	if err != nil {
		return err
	}
	settings.TelemetrySettings.Resource = resource

	// Create instances of the extension from our factory.
	var components []otelcomponent.Component

	// Make sure the component returned a valid set of auth flags.
	authFeature := rargs.AuthFeatures()
	if valid := ValidateAuthFeatures(authFeature); !valid {
		return fmt.Errorf("invalid auth flag %d returned by component %s", authFeature, a.opts.ID)
	}

	// Registers the client extension for the otel collector plugin
	handler := NewHandler(a.opts.ID)
	clientEh, err := a.SetupExtension(Client, rargs, settings)
	if err != nil {
		return err
	}

	// If the extension supports client auth schedule it.
	if HasAuthFeature(authFeature, ClientAuthSupported) {
		components = append(components, clientEh.Extension)
	}

	// Register extension so it can be retrieved when referenced.
	if err := handler.AddExtension(Client, clientEh); err != nil {
		return err
	}

	// Registers server authentication plugin.
	serverEh, err := a.SetupExtension(Server, rargs, settings)
	if err != nil {
		return err
	}

	// If the extension supports server auth schedule it.
	if HasAuthFeature(authFeature, ServerAuthSupported) {
		components = append(components, serverEh.Extension)
	}

	// Register extension so it can be retrieved
	if err := handler.AddExtension(Server, serverEh); err != nil {
		return err
	}

	// Inform listeners that our handler changed.
	a.opts.OnStateChange(Exports{
		Handler: handler,
	},
	)

	// Schedule the components to run once our component is running.
	a.sched.Schedule(a.ctx, func() {}, host, components...)
	return nil
}

// CurrentHealth implements component.HealthComponent.
func (a *Auth) CurrentHealth() component.Health {
	return a.sched.CurrentHealth()
}

func getHash(in string) string {
	fnvHash := fnv.New32()
	fnvHash.Write([]byte(in))
	return fmt.Sprintf("%x", fnvHash.Sum(nil))
}

func NormalizeType(in string) string {
	res := strings.ReplaceAll(strings.ReplaceAll(in, ".", "_"), "/", "_")

	if len(res) > 63 {
		res = res[:40] + getHash(res)
	}

	return res
}

// SetupExtension sets up the extension handler object with the appropriate fields to map the alloy
// capsule to the underlying otel auth extension.
func (a *Auth) SetupExtension(t ExtensionType, rargs Arguments, settings otelextension.Settings) (*ExtensionHandler, error) {
	var otelConfig otelcomponent.Config
	var err error
	var notSupportedErr error
	var requiredAuthFeature AuthFeature

	// Retrieve the appropriate auth extension for the requested type.
	switch t {
	case Server:
		otelConfig, err = rargs.ConvertServer()
		notSupportedErr = ErrNotServerExtension
		requiredAuthFeature = ServerAuthSupported
	case Client:
		otelConfig, err = rargs.ConvertClient()
		notSupportedErr = ErrNotClientExtension
		requiredAuthFeature = ClientAuthSupported
	default:
		return nil, fmt.Errorf("unrecognized extension type %s", t)
	}

	// If there was an error converting the server/client args fail now.
	if err != nil {
		return nil, err
	}

	eh := &ExtensionHandler{}
	extensionAuthFeatures := rargs.AuthFeatures()

	// Auth plugins return a feature flag indicating the types of authentication they support.
	// If the plugin does not support the requested extension type (client or server authentication),
	// the handler will set the error field. This results in an error being triggered if the unsupported
	// extension is accessed via the handler. However, we do not return an error immediately because
	// the user must explicitly request the invalid handler in their configuration for the error to occur.
	// Refer to Handler.GetExtension() for the implementation logic.
	if !HasAuthFeature(extensionAuthFeatures, requiredAuthFeature) {
		eh.Error = fmt.Errorf("%s %w", a.opts.ID, notSupportedErr)
		return eh, nil
	}

	// Create the otel extension via its factory.
	otelExtension, err := a.createExtension(otelConfig, settings)
	if err != nil {
		return nil, err
	}

	// Create an extension id based off the alloy name. For example
	// auth.basic.creds.LABEL will become auth.basic.creds.LABEL.client or auth.basic.creds.LABEL.server
	// depending on the type.
	cTypeStr := NormalizeType(fmt.Sprintf("%s.%s", a.opts.ID, t))
	eh.ID = otelcomponent.NewIDWithName(a.factory.Type(), cTypeStr)
	eh.Extension = otelExtension

	return eh, nil
}

// createExtension uses the otelextension factory to construct the otel auth extension.
func (a *Auth) createExtension(config otelcomponent.Config, settings otelextension.Settings) (otelcomponent.Component, error) {
	ext, err := a.factory.Create(a.ctx, settings, config)
	if err != nil {
		return nil, err
	}

	// sanity check
	if ext == nil {
		return nil, fmt.Errorf("extension was not created")
	}

	return ext, nil
}

// ValidateAuthFeatures makes sure a valid auth feature was returned by a
func ValidateAuthFeatures(f AuthFeature) bool {
	validFlags := ClientAuthSupported | ServerAuthSupported | ClientAndServerAuthSupported

	// bit clear any flags not set in f.
	// if this is not zero then an invalid flag was passed.
	return f&^validFlags == 0
}

func HasAuthFeature(flag AuthFeature, feature AuthFeature) bool {
	// bitwise and the two features together. If not zero it has the feature
	return flag&feature != 0
}
