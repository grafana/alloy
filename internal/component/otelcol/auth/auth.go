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
	"os"
	"strings"

	"github.com/grafana/alloy/internal/build"
	"github.com/grafana/alloy/internal/component"
	otelcolCfg "github.com/grafana/alloy/internal/component/otelcol/config"
	"github.com/grafana/alloy/internal/component/otelcol/internal/lazycollector"
	"github.com/grafana/alloy/internal/component/otelcol/internal/scheduler"
	"github.com/grafana/alloy/internal/util/zapadapter"
	"github.com/grafana/alloy/syntax"
	"github.com/prometheus/client_golang/prometheus"
	otelcomponent "go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configtelemetry"
	otelextension "go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/collector/pipeline"
	sdkprometheus "go.opentelemetry.io/otel/exporters/prometheus"
	otelmetric "go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/noop"
	"go.opentelemetry.io/otel/sdk/metric"
)

var (
	ErrNotServerExtension = errors.New("component does not support server authentication")
	ErrNotClientExtension = errors.New("component does not support client authentication")
	ErrInvalidExtension   = errors.New("invalid extension")
)

type ExtensionType string

const (
	Server ExtensionType = "server"
	Client ExtensionType = "client"
)

// Arguments is an extension of component.Arguments which contains necessary
// settings for OpenTelemetry Collector authentication extensions.
type Arguments interface {
	component.Arguments

	// ConvertClient converts the Arguments into an OpenTelemetry Collector
	// client authentication extension configuration.
	ConvertClient() (otelcomponent.Config, error)

	// ConvetServer converts the Arguments into an OpenTelemetry Collector
	// server authentication extension configuration
	ConvertServer() (otelcomponent.Config, error)

	// Extensions returns the set of extensions that the configured component is
	// allowed to use.
	Extensions() map[otelcomponent.ID]otelextension.Extension

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

func NewHandler(componentID string) *Handler {
	return &Handler{
		componentID: componentID,
		handlerMap:  map[ExtensionType]*ExtensionHandler{},
	}
}

func (h *Handler) GetExtension(et ExtensionType) (*ExtensionHandler, error) {
	ext, ok := h.handlerMap[et]
	if !ok {
		return nil, fmt.Errorf("error initializing %s auth extension. component %s was unexpectedly nil", et, h.componentID)
	}

	if ext.Error != nil {
		return nil, ext.Error
	}

	return ext, nil
}

func (h *Handler) AddExtension(et ExtensionType, eh *ExtensionHandler) error {
	if et != Server && et != Client {
		return fmt.Errorf("invalid extension type %s", et)
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

	metricsLevel, err := rargs.DebugMetricsConfig().Level.Convert()
	if err != nil {
		return err
	}

	mp := metric.NewMeterProvider(metric.WithReader(promExporter))
	settings := otelextension.Settings{
		TelemetrySettings: otelcomponent.TelemetrySettings{
			Logger: zapadapter.New(a.opts.Logger),

			TracerProvider: a.opts.Tracer,
			MeterProvider:  mp,
			LeveledMeterProvider: func(level configtelemetry.Level) otelmetric.MeterProvider {
				if level <= metricsLevel {
					return mp
				}
				return noop.MeterProvider{}
			},
			MetricsLevel: metricsLevel,
		},

		BuildInfo: otelcomponent.BuildInfo{
			Command:     os.Args[0],
			Description: "Grafana Alloy",
			Version:     build.Version,
		},
	}

	// Create instances of the extension from our factory.
	var components []otelcomponent.Component
	handler := NewHandler(a.opts.ID)
	clientEh, err := a.setupExtension(Client, rargs, settings)
	if err != nil {
		return err
	}

	// Extension could be nil if the auth plugin does not support client auth
	if clientEh.Extension != nil {
		components = append(components, clientEh.Extension)
	}

	// Register extension so it can be retrieved
	if err := handler.AddExtension(Client, clientEh); err != nil {
		return err
	}

	serverEh, err := a.setupExtension(Server, rargs, settings)
	if err != nil {
		return err
	}

	// Extension could be nil if the auth plugin does not support server auth.
	if serverEh.Extension != nil {
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
	a.sched.Schedule(host, components...)
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

func (a *Auth) setupExtension(t ExtensionType, rargs Arguments, settings otelextension.Settings) (*ExtensionHandler, error) {
	var otelConfig otelcomponent.Config
	var err error
	var notSupportedErr error
	if t == Server {
		otelConfig, err = rargs.ConvertServer()
		notSupportedErr = ErrNotServerExtension
	}
	if t == Client {
		otelConfig, err = rargs.ConvertClient()
		notSupportedErr = ErrNotClientExtension
	}

	if err != nil {
		return nil, err
	}

	eh := &ExtensionHandler{}

	// Auth plugins that don't support the client/server auth
	// are expected to return nil, check for that error here.
	if otelConfig == nil {
		eh.Error = fmt.Errorf("%s %w", a.opts.ID, notSupportedErr)
		return eh, nil
	}

	otelExtension, err := a.createExtension(otelConfig, settings)
	if err != nil {
		return nil, err
	}

	cTypeStr := NormalizeType(fmt.Sprintf("%s.%s", a.opts.ID, t))
	eh.ID = otelcomponent.NewID(otelcomponent.MustNewType(cTypeStr))
	eh.Extension = otelExtension

	return eh, nil
}

func (a *Auth) createExtension(config otelcomponent.Config, settings otelextension.Settings) (otelcomponent.Component, error) {
	ext, err := a.factory.Create(a.ctx, settings, config)
	if err != nil {
		return nil, err
	}
	if ext == nil {
		return nil, fmt.Errorf("extension was not created")
	}

	return ext, nil
}
