package scrape

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"
	"go.uber.org/goleak"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/component/prometheus/scrape"
	"github.com/grafana/alloy/internal/component/pyroscope"
	"github.com/grafana/alloy/internal/service/cluster"
	http_service "github.com/grafana/alloy/internal/service/http"
	"github.com/grafana/alloy/internal/util"
	"github.com/grafana/alloy/syntax"
)

func TestComponent(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreTopFunction("go.opencensus.io/stats/view.(*worker).start"))
	reloadInterval = 100 * time.Millisecond
	arg := NewDefaultArguments()
	arg.JobName = "test"
	c, err := New(component.Options{
		Logger:         util.TestAlloyLogger(t),
		Registerer:     prometheus.NewRegistry(),
		OnStateChange:  func(e component.Exports) {},
		GetServiceData: getServiceData,
	}, arg)
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	go func() {
		err := c.Run(ctx)
		require.NoError(t, err)
	}()

	// trigger an update
	require.Empty(t, c.appendable.Children())
	require.Empty(t, c.DebugInfo().(scrape.ScraperStatus).TargetStatus)

	arg.ForwardTo = []pyroscope.Appendable{pyroscope.NoopAppendable}
	arg.Targets = []discovery.Target{
		discovery.NewTargetFromMap(map[string]string{
			model.AddressLabel: "foo",
			serviceNameLabel:   "s",
		}),
		discovery.NewTargetFromMap(map[string]string{
			model.AddressLabel:  "bar",
			serviceNameK8SLabel: "k",
		}),
	}
	c.Update(arg)

	require.Eventually(t, func() bool {
		fmt.Println(c.DebugInfo().(scrape.ScraperStatus).TargetStatus)
		return len(c.appendable.Children()) == 1 && len(c.DebugInfo().(scrape.ScraperStatus).TargetStatus) == 10
	}, 5*time.Second, 100*time.Millisecond)
}

func getServiceData(name string) (any, error) {
	switch name {
	case cluster.ServiceName:
		return cluster.Mock(), nil
	case http_service.ServiceName:
		return http_service.Data{
			HTTPListenAddr:   "localhost:12345",
			MemoryListenAddr: "alloy.internal:1245",
			BaseHTTPPath:     "/",
			DialFunc:         (&net.Dialer{}).DialContext,
		}, nil
	default:
		return nil, fmt.Errorf("unrecognized service name %q", name)
	}
}

func TestUnmarshalConfig(t *testing.T) {
	for name, tt := range map[string]struct {
		in          string
		expected    func() Arguments
		expectedErr string
	}{
		"default": {
			in: `
			targets    = [
				{"__address__" = "localhost:9090", "foo" = "bar"},
			]
			forward_to = null
		   `,
			expected: func() Arguments {
				r := NewDefaultArguments()
				r.Targets = []discovery.Target{
					discovery.NewTargetFromMap(map[string]string{
						"__address__": "localhost:9090",
						"foo":         "bar",
					}),
				}
				return r
			},
		},
		"custom": {
			in: `
			targets    = [
				{"__address__" = "localhost:9090", "foo" = "bar"},
				{"__address__" = "localhost:8080", "foo" = "buzz"},
			]
			forward_to = null
			profiling_config {
				path_prefix = "v1/"

				profile.block {
					enabled = false
				}

				profile.custom "something" {
					enabled = true
					path    = "/debug/fgprof"
					delta   = true
				}
		   }
		   `,
			expected: func() Arguments {
				r := NewDefaultArguments()
				r.Targets = []discovery.Target{
					discovery.NewTargetFromMap(map[string]string{
						"__address__": "localhost:9090",
						"foo":         "bar",
					}),
					discovery.NewTargetFromMap(map[string]string{
						"__address__": "localhost:8080",
						"foo":         "buzz",
					}),
				}
				r.ProfilingConfig.Block.Enabled = false
				r.ProfilingConfig.Custom = append(r.ProfilingConfig.Custom, CustomProfilingTarget{
					Enabled: true,
					Path:    "/debug/fgprof",
					Delta:   true,
					Name:    "something",
				})
				r.ProfilingConfig.PathPrefix = "v1/"
				return r
			},
		},
		"invalid cpu scrape_interval": {
			in: `
			targets    = []
			forward_to = null
			scrape_timeout = "1s"
			scrape_interval = "0.5s"
			`,
			expectedErr: "scrape_interval must be at least 2 seconds when using delta profiling",
		},
		"invalid cpu delta_profiling_duration": {
			in: `
			targets    = []
			forward_to = null
			scrape_timeout = "1s"
			scrape_interval = "10s"
			delta_profiling_duration = "1s"
			`,
			expectedErr: "delta_profiling_duration must be larger than 1 second when using delta profiling",
		},
		"erroneous cpu delta_profiling_duration": {
			in: `
			targets    = []
			forward_to = null
			scrape_timeout = "1s"
			scrape_interval = "10s"
			delta_profiling_duration = "12s"
			`,
			expectedErr: "delta_profiling_duration must be at least 1 second smaller than scrape_interval when using delta profiling",
		},
		"allow short scrape_intervals without delta": {
			in: `
			targets    = []
			forward_to = null
			scrape_interval = "0.5s"
			profiling_config {
				profile.process_cpu {
					enabled = false
				}
		   }
			`,
			expected: func() Arguments {
				r := NewDefaultArguments()
				r.Targets = make([]discovery.Target, 0)
				r.ScrapeInterval = 500 * time.Millisecond
				r.ProfilingConfig.ProcessCPU.Enabled = false
				return r
			},
		},
		"invalid HTTPClientConfig": {
			in: `
			targets    = []
			forward_to = null
			scrape_timeout = "5s"
			scrape_interval = "3s"
			delta_profiling_duration = "2s"
			bearer_token = "token"
			bearer_token_file = "/path/to/file.token"
			`,
			expectedErr: "at most one of basic_auth, authorization, oauth2, bearer_token & bearer_token_file must be configured",
		},
	} {
		tt := tt
		name := name
		t.Run(name, func(t *testing.T) {
			arg := Arguments{}
			if tt.expectedErr != "" {
				err := syntax.Unmarshal([]byte(tt.in), &arg)
				require.Error(t, err)
				require.Equal(t, tt.expectedErr, err.Error())
				return
			}
			require.NoError(t, syntax.Unmarshal([]byte(tt.in), &arg))
			require.Equal(t, tt.expected(), arg)
		})
	}
}

func TestUpdateWhileScraping(t *testing.T) {
	args := NewDefaultArguments()
	// speed up reload interval for this tests
	old := reloadInterval
	reloadInterval = 1 * time.Microsecond
	defer func() {
		reloadInterval = old
	}()
	args.ScrapeInterval = 1 * time.Second

	c, err := New(component.Options{
		Logger:         util.TestAlloyLogger(t),
		Registerer:     prometheus.NewRegistry(),
		OnStateChange:  func(e component.Exports) {},
		GetServiceData: getServiceData,
	}, args)
	require.NoError(t, err)
	scraping := atomic.NewBool(false)
	ctx, cancel := context.WithCancel(t.Context())

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		scraping.Store(true)
		select {
		case <-ctx.Done():
			return
		case <-time.After(15 * time.Second):
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	address := strings.TrimPrefix(server.URL, "http://")

	defer cancel()

	go c.Run(ctx)

	args.Targets = []discovery.Target{
		discovery.NewTargetFromMap(map[string]string{
			model.AddressLabel: address,
			serviceNameLabel:   "s",
			"foo":              "bar",
		}),
		discovery.NewTargetFromMap(map[string]string{
			model.AddressLabel:  address,
			serviceNameK8SLabel: "k",
			"foo":               "buz",
		}),
	}

	c.Update(args)
	c.scraper.reload()
	// Wait for the targets to be scraping.
	require.Eventually(t, func() bool {
		return scraping.Load()
	}, 10*time.Second, 1*time.Second)

	// Send updates to the targets.
	done := make(chan struct{})
	go func() {
		for i := 0; i < 100; i++ {
			args.Targets = []discovery.Target{
				discovery.NewTargetFromMap(map[string]string{
					model.AddressLabel: address,
					serviceNameLabel:   "s",
					"foo":              fmt.Sprintf("%d", i),
				}),
			}
			require.NoError(t, c.Update(args))
			c.scraper.reload()
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for updates to finish")
	}
}
