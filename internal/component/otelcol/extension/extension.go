// Package extension provides utilities to create an Alloy component from
// OpenTelemetry Collector extensions.
//
// Other OpenTelemetry Collector extensions are better served as generic Alloy
// components rather than being placed in the otelcol namespace.
package extension

import (
	"context"

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

// Arguments is an extension of component.Arguments which contains necessary
// settings for OpenTelemetry Collector extensions.
type Arguments interface {
	component.Arguments

	// Convert converts the Arguments into an OpenTelemetry Collector
	// extension configuration.
	Convert(component.Options) (otelcomponent.Config, error)

	// Extensions returns the set of extensions that the configured component is
	// allowed to use.
	Extensions() map[otelcomponent.ID]otelcomponent.Component

	// Exporters returns the set of exporters that are exposed to the configured
	// component.
	Exporters() map[pipeline.Signal]map[otelcomponent.ID]otelcomponent.Component

	// DebugMetricsConfig returns the configuration for debug metrics
	DebugMetricsConfig() otelcolCfg.DebugMetricsArguments

	// ExportsHandler returns a boolean indicating whether the component
	// should export the handler for other components to reference.
	ExportsHandler() bool
}

// Exports is a common Exports type for Alloy components which expose
// OpenTelemetry Collector storage extensions.
type Exports struct {
	// Handler is the managed component. Handler is updated any time the
	// extension is updated.
	Handler *ExtensionHandler `alloy:"handler,attr"`
}

type ExtensionHandler struct {
	ID        otelcomponent.ID
	Extension otelextension.Extension
}

var _ syntax.Capsule = ExtensionHandler{}

// AlloyCapsule marks Handler as a capsule type.
func (ExtensionHandler) AlloyCapsule() {}

// Extension is an Alloy component shim which manages an OpenTelemetry
// Collector extension.
type Extension struct {
	ctx    context.Context
	cancel context.CancelFunc

	opts    component.Options
	factory otelextension.Factory

	sched     *scheduler.Scheduler
	collector *lazycollector.Collector
}

var (
	_ component.Component       = (*Extension)(nil)
	_ component.HealthComponent = (*Extension)(nil)
)

// New creates a new Alloy component which encapsulates an OpenTelemetry
// Collector extension. args must hold a value of the argument
// type registered with the Alloy component.
func New(opts component.Options, f otelextension.Factory, args Arguments) (*Extension, error) {
	ctx, cancel := context.WithCancel(context.Background())

	// Create a lazy collector where metrics from the upstream component will be
	// forwarded.
	collector := lazycollector.New()
	opts.Registerer.MustRegister(collector)

	r := &Extension{
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

// Run starts the Extension component.
func (e *Extension) Run(ctx context.Context) error {
	e.opts.Logger.Log("level", "info", "msg", "starting extension", "component", e.opts.ID)
	defer e.cancel()
	return e.sched.Run(ctx)
}

// Update implements component.Component. It will convert the Arguments into
// configuration for OpenTelemetry Collector extension
// configuration and manage the underlying OpenTelemetry Collector extension.
func (e *Extension) Update(args component.Arguments) error {
	rargs := args.(Arguments)

	host := scheduler.NewHost(
		e.opts.Logger,
		scheduler.WithHostExtensions(rargs.Extensions()),
		scheduler.WithHostExporters(rargs.Exporters()),
	)

	reg := prometheus.NewRegistry()
	e.collector.Set(reg)

	promExporter, err := sdkprometheus.New(sdkprometheus.WithRegisterer(reg), sdkprometheus.WithoutTargetInfo())
	if err != nil {
		return err
	}

	mp := metric.NewMeterProvider(metric.WithReader(promExporter))
	settings := otelextension.Settings{
		ID: otelcomponent.NewIDWithName(e.factory.Type(), e.opts.ID),
		TelemetrySettings: otelcomponent.TelemetrySettings{
			Logger:         zapadapter.New(e.opts.Logger),
			TracerProvider: e.opts.Tracer,
			MeterProvider:  mp,
		},

		BuildInfo: otelcolutil.GetBuildInfo(),
	}

	resource, err := otelcolutil.GetTelemetrySettingsResource()
	if err != nil {
		return err
	}
	settings.TelemetrySettings.Resource = resource

	extensionConfig, err := rargs.Convert(e.opts)
	if err != nil {
		return err
	}

	// Create instances of the extension from our factory.
	var components []otelcomponent.Component

	ext, err := e.factory.Create(e.ctx, settings, extensionConfig)
	if err != nil {
		return err
	} else if ext != nil {
		components = append(components, ext)
	}

	if rargs.ExportsHandler() {
		// Registers the extension for the otel collector plugin
		handler := &ExtensionHandler{
			ID:        settings.ID,
			Extension: ext,
		}

		// Inform listeners that our handler changed.
		e.opts.OnStateChange(Exports{
			Handler: handler,
		})
	}

	// Schedule the components to run once our component is running.
	e.sched.Schedule(e.ctx, func() {}, host, components...)
	return nil
}

// CurrentHealth implements component.HealthComponent.
func (e *Extension) CurrentHealth() component.Health {
	return e.sched.CurrentHealth()
}
