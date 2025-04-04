// Package storage provides utilities to create an Alloy component from
// OpenTelemetry Collector storage extensions.
package storage

import (
	"context"
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
	otelextension "go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/collector/pipeline"
	sdkprometheus "go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/sdk/metric"
)

// Arguments is an extension of component.Arguments which contains necessary
// settings for OpenTelemetry Collector storage extensions.
type Arguments interface {
	component.Arguments

	// Convert converts the Arguments into an OpenTelemetry Collector
	// storage extension configuration.
	Convert() (otelcomponent.Config, error)

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
// OpenTelemetry Collector storage extensions.
type Exports struct {
	// Handler is the managed component. Handler is updated any time the
	// extension is updated.
	Handler *ExtensionHandler `alloy:"handler,attr"`
}

type ExtensionHandler struct {
	ID        otelcomponent.ID
	Extension otelextension.Extension

	componentID string
}

func NewHandler(componentID string) *ExtensionHandler {
	return &ExtensionHandler{
		componentID: componentID,
	}
}

var _ syntax.Capsule = ExtensionHandler{}

// AlloyCapsule marks Handler as a capsule type.
func (ExtensionHandler) AlloyCapsule() {}

// Storage is an Alloy component shim which manages an OpenTelemetry Collector
// storage extension.
type Storage struct {
	ctx    context.Context
	cancel context.CancelFunc

	opts    component.Options
	factory otelextension.Factory

	sched     *scheduler.Scheduler
	collector *lazycollector.Collector
}

var (
	_ component.Component       = (*Storage)(nil)
	_ component.HealthComponent = (*Storage)(nil)
)

// New creates a new Alloy component which encapsulates an OpenTelemetry
// Collector storage extension. args must hold a value of the argument
// type registered with the Alloy component.
//
// The registered component must be registered to export the Exports type from
// this package, otherwise New will panic.
func New(opts component.Options, f otelextension.Factory, args Arguments) (*Storage, error) {
	ctx, cancel := context.WithCancel(context.Background())

	// Create a lazy collector where metrics from the upstream component will be
	// forwarded.
	collector := lazycollector.New()
	opts.Registerer.MustRegister(collector)

	r := &Storage{
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

// Run starts the Storage component.
func (s *Storage) Run(ctx context.Context) error {
	defer s.cancel()
	return s.sched.Run(ctx)
}

// Update implements component.Component. It will convert the Arguments into
// configuration for OpenTelemetry Collector storage extension
// configuration and manage the underlying OpenTelemetry Collector extension.
func (s *Storage) Update(args component.Arguments) error {
	rargs := args.(Arguments)

	host := scheduler.NewHost(
		s.opts.Logger,
		scheduler.WithHostExtensions(rargs.Extensions()),
		scheduler.WithHostExporters(rargs.Exporters()),
	)

	reg := prometheus.NewRegistry()
	s.collector.Set(reg)

	promExporter, err := sdkprometheus.New(sdkprometheus.WithRegisterer(reg), sdkprometheus.WithoutTargetInfo())
	if err != nil {
		return err
	}

	mp := metric.NewMeterProvider(metric.WithReader(promExporter))
	settings := otelextension.Settings{
		ID: otelcomponent.NewID(s.factory.Type()),
		TelemetrySettings: otelcomponent.TelemetrySettings{
			Logger: zapadapter.New(s.opts.Logger),

			TracerProvider: s.opts.Tracer,
			MeterProvider:  mp,
		},

		BuildInfo: otelcomponent.BuildInfo{
			Command:     os.Args[0],
			Description: "Grafana Alloy",
			Version:     build.Version,
		},
	}

	// Registers the extension for the otel collector plugin
	handler, err := s.SetupExtension(rargs, settings)
	if err != nil {
		return err
	}

	// Inform listeners that our handler changed.
	s.opts.OnStateChange(Exports{
		Handler: handler,
	})

	// Schedule the component to run once our component is running.
	s.sched.Schedule(s.ctx, func() {}, host, handler.Extension)
	return nil
}

// CurrentHealth implements component.HealthComponent.
func (s *Storage) CurrentHealth() component.Health {
	return s.sched.CurrentHealth()
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
// capsule to the underlying otel storage extension.
func (s *Storage) SetupExtension(rargs Arguments, settings otelextension.Settings) (*ExtensionHandler, error) {
	handler := &ExtensionHandler{}

	otelConfig, err := rargs.Convert()
	if err != nil {
		return nil, err
	}

	// Create the otel extension via its factory.
	otelExtension, err := s.factory.Create(s.ctx, settings, otelConfig)
	if err != nil {
		return nil, err
	}

	// sanity check
	if otelExtension == nil {
		return nil, fmt.Errorf("extension was not created")
	}

	// Create an extension id based off the alloy name
	cTypeStr := NormalizeType(s.opts.ID)
	handler.ID = otelcomponent.NewID(otelcomponent.MustNewType(cTypeStr))
	handler.Extension = otelExtension

	return handler, nil
}
