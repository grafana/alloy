package traceutils

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"testing"
	"time"

	"github.com/grafana/alloy/internal/util"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/connector"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/exporter"
	otelexporter "go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/collector/otelcol"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/otlpreceiver"
	"go.opentelemetry.io/collector/service"
	"go.opentelemetry.io/otel/trace/noop"
	"gopkg.in/yaml.v3"
)

// server is a Tracing testing server that invokes a function every time a span
// is received.
type server struct {
	service *service.Service
}

// NewTestServer creates a new server for testing, where received traces will
// call the callback function. The returned string is the address where traces
// can be sent using OTLP.
func NewTestServer(t *testing.T, callback func(ptrace.Traces)) string {
	t.Helper()

	srv, listenAddr, err := newServerWithRandomPort(callback)
	if err != nil {
		t.Fatalf("failed to create OTLP server: %s", err)
	}
	t.Cleanup(func() {
		err := srv.stop()
		assert.NoError(t, err)
	})

	return listenAddr
}

// newServerWithRandomPort calls NewServer with a random port >49152 and
// <65535. It will try up to five times before failing.
func newServerWithRandomPort(callback func(ptrace.Traces)) (srv *server, addr string, err error) {
	var lastError error

	for i := 0; i < 5; i++ {
		port := rand.Intn(65535-49152) + 49152
		listenAddr := fmt.Sprintf("127.0.0.1:%d", port)

		srv, err = newServer(listenAddr, callback)
		if err != nil {
			lastError = err
			continue
		}

		return srv, listenAddr, nil
	}

	return nil, "", fmt.Errorf("failed 5 times to create a server. last error: %w", lastError)
}

// newServer creates an OTLP-accepting server that calls a function when a
// trace is received. This is primarily useful for testing.
func newServer(addr string, callback func(ptrace.Traces)) (*server, error) {
	conf := util.Untab(fmt.Sprintf(`
	processors:
		func_processor:
	receivers:
		otlp:
			protocols:
				grpc:
					endpoint: %s
	exporters:
		noop:
	service:
		pipelines:
			traces:
				receivers: [otlp]
				processors: [func_processor]
				exporters: [noop]
	`, addr))

	var cfg map[string]interface{}
	if err := yaml.NewDecoder(strings.NewReader(conf)).Decode(&cfg); err != nil {
		panic("could not decode config: " + err.Error())
	}

	extensionsFactory, err := extension.MakeFactoryMap()
	if err != nil {
		return nil, fmt.Errorf("failed to make extension factory map: %w", err)
	}

	receiversFactory, err := receiver.MakeFactoryMap(otlpreceiver.NewFactory())
	if err != nil {
		return nil, fmt.Errorf("failed to make receiver factory map: %w", err)
	}

	exportersFactory, err := exporter.MakeFactoryMap(newNoopExporterFactory())
	if err != nil {
		return nil, fmt.Errorf("failed to make exporter factory map: %w", err)
	}

	processorsFactory, err := processor.MakeFactoryMap(
		newFuncProcessorFactory(callback),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to make processor factory map: %w", err)
	}

	factories := otelcol.Factories{
		Extensions: extensionsFactory,
		Receivers:  receiversFactory,
		Processors: processorsFactory,
		Exporters:  exportersFactory,
	}

	configMap := confmap.NewFromStringMap(cfg)
	otelCfgSettings, err := otelcol.Unmarshal(configMap, factories)
	if err != nil {
		return nil, fmt.Errorf("failed to make otel config: %w", err)
	}

	otelCfg := otelcol.Config{
		Receivers:  otelCfgSettings.Receivers.Configs(),
		Processors: otelCfgSettings.Processors.Configs(),
		Exporters:  otelCfgSettings.Exporters.Configs(),
		Connectors: otelCfgSettings.Connectors.Configs(),
		Extensions: otelCfgSettings.Extensions.Configs(),
		Service:    otelCfgSettings.Service,
	}

	if err := otelCfg.Validate(); err != nil {
		return nil, err
	}

	svc, err := service.New(context.Background(), service.Settings{
		Receivers:      receiver.NewBuilder(otelCfg.Receivers, factories.Receivers),
		Processors:     processor.NewBuilder(otelCfg.Processors, factories.Processors),
		Exporters:      otelexporter.NewBuilder(otelCfg.Exporters, factories.Exporters),
		Connectors:     connector.NewBuilder(otelCfg.Connectors, factories.Connectors),
		Extensions:     extension.NewBuilder(otelCfg.Extensions, factories.Extensions),
		TracerProvider: noop.NewTracerProvider(),
	}, otelCfg.Service)
	if err != nil {
		return nil, fmt.Errorf("failed to create Otel service: %w", err)
	}

	if err := svc.Start(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to start Otel service: %w", err)
	}

	return &server{
		service: svc,
	}, nil
}

// stop stops the testing server.
func (s *server) stop() error {
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	return s.service.Shutdown(shutdownCtx)
}

func newFuncProcessorFactory(callback func(ptrace.Traces)) processor.Factory {
	return processor.NewFactory(
		component.MustNewType("func_processor"),
		func() component.Config {
			return &struct{}{}
		},
		processor.WithTraces(func(
			_ context.Context,
			_ processor.Settings,
			_ component.Config,
			next consumer.Traces,
		) (processor.Traces, error) {

			return &funcProcessor{
				Callback: callback,
				Next:     next,
			}, nil
		}, component.StabilityLevelUndefined),
	)
}

type funcProcessor struct {
	Callback func(ptrace.Traces)
	Next     consumer.Traces
}

func (p *funcProcessor) ConsumeTraces(ctx context.Context, td ptrace.Traces) error {
	if p.Callback != nil {
		p.Callback(td)
	}
	return p.Next.ConsumeTraces(ctx, td)
}

func (p *funcProcessor) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: true}
}

func (p *funcProcessor) Start(context.Context, component.Host) error { return nil }
func (p *funcProcessor) Shutdown(context.Context) error              { return nil }

func newNoopExporterFactory() exporter.Factory {
	return exporter.NewFactory(
		component.MustNewType("noop"),
		func() component.Config {
			return &struct{}{}
		},
		exporter.WithTraces(func(
			context.Context,
			exporter.Settings,
			component.Config) (
			exporter.Traces,
			error) {

			return &noopExporter{}, nil
		}, component.StabilityLevelUndefined),
	)
}

type noopExporter struct{}

func (n noopExporter) Start(context.Context, component.Host) error { return nil }

func (n noopExporter) Shutdown(context.Context) error { return nil }

func (n noopExporter) Capabilities() consumer.Capabilities { return consumer.Capabilities{} }

func (n noopExporter) ConsumeTraces(context.Context, ptrace.Traces) error { return nil }
