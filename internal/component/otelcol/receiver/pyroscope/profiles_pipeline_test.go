package pyroscope

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	httppprof "net/http/pprof"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/prometheus/model/labels"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pprofile"
	"go.opentelemetry.io/collector/pdata/pprofile/pprofileotlp"
	"google.golang.org/grpc"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/component/otelcol"
	exporterotlp "github.com/grafana/alloy/internal/component/otelcol/exporter/otlp"
	"github.com/grafana/alloy/internal/component/otelcol/processor/memorylimiter"
	alloypyroscope "github.com/grafana/alloy/internal/component/pyroscope"
	"github.com/grafana/alloy/internal/component/pyroscope/scrape"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/runtime/componenttest"
	"github.com/grafana/alloy/internal/service/cluster"
	httpservice "github.com/grafana/alloy/internal/service/http"
	"github.com/grafana/alloy/internal/util"
	"github.com/grafana/alloy/syntax"
)

type profilesPipelineSink struct {
	pprofileotlp.UnimplementedGRPCServer
	profiles chan pprofile.Profiles
}

func (s *profilesPipelineSink) Export(ctx context.Context, req pprofileotlp.ExportRequest) (pprofileotlp.ExportResponse, error) {
	select {
	case s.profiles <- req.Profiles():
		return pprofileotlp.NewExportResponse(), nil
	case <-ctx.Done():
		return pprofileotlp.NewExportResponse(), ctx.Err()
	}
}

func TestProfilesPipeline(t *testing.T) {
	for _, method := range []string{"append", "ingest"} {
		t.Run(method, func(t *testing.T) {
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			require.NoError(t, err)
			sink := &profilesPipelineSink{profiles: make(chan pprofile.Profiles, 1)}
			server := grpc.NewServer()
			pprofileotlp.RegisterGRPCServer(server, sink)
			go func() { _ = server.Serve(listener) }()
			t.Cleanup(server.Stop)

			var exporterArgs exporterotlp.Arguments
			require.NoError(t, syntax.Unmarshal([]byte(fmt.Sprintf(`
				client {
					endpoint = %q
					tls { insecure = true }
				}
				sending_queue { enabled = false }
				retry_on_failure { enabled = false }
			`, listener.Addr().String())), &exporterArgs))
			exporter := runPipelineComponent(t, "otelcol.exporter.otlp", exporterArgs)

			var processorArgs memorylimiter.Arguments
			require.NoError(t, syntax.Unmarshal([]byte(`
				check_interval = "1h"
				limit = "1GiB"
				output {}
			`), &processorArgs))
			processorArgs.Output.Profiles = []otelcol.Consumer{exporter.Exports().(otelcol.ConsumerExports).Input}
			processor := runPipelineComponent(t, "otelcol.processor.memory_limiter", processorArgs)

			bridge := runPipelineComponent(t, "otelcol.receiver.pyroscope", Arguments{
				Output: &otelcol.ConsumerArguments{
					Profiles: []otelcol.Consumer{processor.Exports().(otelcol.ConsumerExports).Input},
				},
			})
			// Use the same fanout appendable as the Pyroscope source components.
			fanout := alloypyroscope.NewFanout([]alloypyroscope.Appendable{bridge.Exports().(Exports).Receiver}, "test", bridge.PromRegistry)
			lbs := labels.FromStrings(
				alloypyroscope.LabelName, "process_cpu:cpu:nanoseconds:cpu:nanoseconds",
				alloypyroscope.LabelServiceName, "checkout-api",
				alloypyroscope.LabelOtelScopeName, alloypyroscope.ScopeNameEBPF,
				alloypyroscope.LabelOtelScopeVersion, "v1.2.3",
				"environment", "production",
			)
			ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
			defer cancel()
			if method == "ingest" {
				err = fanout.Appender().AppendIngest(ctx, &alloypyroscope.IncomingProfile{
					RawBody:     testPprof(t, 42),
					ContentType: []string{"application/octet-stream"},
					URL:         &url.URL{Path: "/ingest", RawQuery: "format=pprof"},
					Labels:      lbs,
				})
			} else {
				err = fanout.Appender().Append(ctx, lbs, []*alloypyroscope.RawSample{{RawProfile: testPprof(t, 42)}})
			}
			require.NoError(t, err)

			select {
			case got := <-sink.profiles:
				assertConvertedProfiles(t, got)
			case <-ctx.Done():
				t.Fatal("converted profiles did not reach the OTLP exporter")
			}
		})
	}
}

// Exercise a real source as well as the appender entry points above. The live
// profile checks that conversion works with Go's profiler, beyond our fixture.
func TestProfilesScrapePipeline(t *testing.T) {
	for _, live := range []bool{false, true} {
		name := "fixture"
		if live {
			name = "live CPU profile"
		}
		t.Run(name, func(t *testing.T) {
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			require.NoError(t, err)
			sink := &profilesPipelineSink{profiles: make(chan pprofile.Profiles, 1)}
			server := grpc.NewServer()
			pprofileotlp.RegisterGRPCServer(server, sink)
			go func() { _ = server.Serve(listener) }()
			t.Cleanup(server.Stop)

			var exporterArgs exporterotlp.Arguments
			require.NoError(t, syntax.Unmarshal([]byte(fmt.Sprintf(`
				client {
					endpoint = %q
					tls { insecure = true }
				}
				sending_queue { enabled = false }
				retry_on_failure { enabled = false }
			`, listener.Addr().String())), &exporterArgs))
			exporter := runPipelineComponent(t, "otelcol.exporter.otlp", exporterArgs)
			bridge := runPipelineComponent(t, "otelcol.receiver.pyroscope", Arguments{
				Output: &otelcol.ConsumerArguments{
					Profiles: []otelcol.Consumer{exporter.Exports().(otelcol.ConsumerExports).Input},
				},
			})

			fixture := testPprof(t, 42)
			source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/debug/pprof/profile" {
					http.NotFound(w, r)
					return
				}
				if live {
					httppprof.Profile(w, r)
					return
				}
				w.Header().Set("Content-Type", "application/octet-stream")
				_, _ = w.Write(fixture)
			}))
			t.Cleanup(source.Close)

			ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
			defer cancel()
			if live {
				done := make(chan struct{})
				go func() {
					defer close(done)
					n := uint64(1)
					for ctx.Err() == nil {
						n = burnProfileCPU(n)
					}
				}()
				defer func() { cancel(); <-done }()
			}

			var scraperArgs scrape.Arguments
			require.NoError(t, syntax.Unmarshal([]byte(`
				targets = []
				forward_to = []
				scrape_interval = "3s"
				scrape_timeout = "3s"
				delta_profiling_duration = "2s"
				profiling_config {
					profile.memory { enabled = false }
					profile.block { enabled = false }
					profile.goroutine { enabled = false }
					profile.mutex { enabled = false }
					profile.process_cpu { enabled = true }
				}
			`), &scraperArgs))
			scraperArgs.Targets = []discovery.Target{discovery.NewTargetFromMap(map[string]string{
				"__address__":  strings.TrimPrefix(source.URL, "http://"),
				"service_name": "checkout-api",
				"environment":  "production",
			})}
			scraperArgs.ForwardTo = []alloypyroscope.Appendable{bridge.Exports().(Exports).Receiver}
			runPipelineComponent(t, "pyroscope.scrape", scraperArgs, func(opts component.Options) component.Options {
				getServiceData := opts.GetServiceData
				opts.GetServiceData = func(name string) (any, error) {
					switch name {
					case httpservice.ServiceName:
						return httpservice.Data{DialFunc: (&net.Dialer{}).DialContext}, nil
					case cluster.ServiceName:
						return cluster.Mock(), nil
					default:
						return getServiceData(name)
					}
				}
				return opts
			})

			select {
			case got := <-sink.profiles:
				require.Positive(t, got.SampleCount())
				rp := got.ResourceProfiles().At(0)
				attrs := rp.Resource().Attributes().AsRaw()
				require.Equal(t, "checkout-api", attrs["service.name"])
				require.Equal(t, "production", attrs["environment"])
				require.Equal(t, "process_cpu", attrs["__name__"])
				require.NotContains(t, attrs, "__address__")
				require.NotContains(t, attrs, "service_name")
				sp := rp.ScopeProfiles().At(0)
				require.Equal(t, alloypyroscope.ScopeNameScrape, sp.Scope().Name())
				dictionary := got.Dictionary()
				var positiveCPU bool
				for _, prof := range sp.Profiles().All() {
					if dictionary.StringTable().At(int(prof.SampleType().TypeStrindex())) != "cpu" {
						continue
					}
					require.Equal(t, "nanoseconds", dictionary.StringTable().At(int(prof.SampleType().UnitStrindex())))
					for _, sample := range prof.Samples().All() {
						for _, value := range sample.Values().All() {
							positiveCPU = positiveCPU || value > 0
							if !live {
								require.Equal(t, int64(42), value)
							}
						}
					}
				}
				require.True(t, positiveCPU, "export must contain positive CPU samples")
				var foundFunction bool
				for _, function := range dictionary.FunctionTable().All() {
					name := dictionary.StringTable().At(int(function.NameStrindex()))
					if live {
						foundFunction = foundFunction || strings.HasSuffix(name, ".burnProfileCPU")
					} else {
						foundFunction = foundFunction || name == "main.checkout"
					}
				}
				require.True(t, foundFunction, "profile lost the workload function")
			case <-ctx.Done():
				t.Fatal("scraped profiles did not reach the OTLP exporter")
			}
		})
	}
}

//go:noinline
func burnProfileCPU(n uint64) uint64 {
	for range 1_000_000 {
		n = n*6364136223846793005 + 1442695040888963407
	}
	return n
}

func runPipelineComponent(t *testing.T, name string, args component.Arguments, modifiers ...func(component.Options) component.Options) *componenttest.Controller {
	t.Helper()
	ctrl, err := componenttest.NewControllerFromID(util.TestLogger(t), name)
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() {
		done <- ctrl.Run(ctx, args, func(opts component.Options) component.Options {
			opts.MinStability = featuregate.StabilityExperimental
			for _, modify := range modifiers {
				opts = modify(opts)
			}
			return opts
		})
	}()
	t.Cleanup(func() {
		cancel()
		select {
		case err := <-done:
			require.NoError(t, err)
		case <-time.After(10 * time.Second):
			t.Errorf("%s did not stop", name)
		}
	})
	require.NoError(t, ctrl.WaitRunning(5*time.Second))
	reg, ok := component.Get(name)
	require.True(t, ok)
	if reg.Exports != nil {
		require.NoError(t, ctrl.WaitExports(5*time.Second))
	}
	return ctrl
}
