package route

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/value"
	"github.com/prometheus/prometheus/storage"
	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/component/prometheus"
	"github.com/grafana/alloy/internal/component/prometheus/scrape"
	"github.com/grafana/alloy/internal/runtime/componenttest"
	"github.com/grafana/alloy/internal/service/cluster"
	httpservice "github.com/grafana/alloy/internal/service/http"
	"github.com/grafana/alloy/internal/service/labelstore"
	"github.com/grafana/alloy/internal/util"
)

// Scrape's staleness tracking requires distinct nonzero refs for series accepted
// downstream. Verify the actual scrape loop, including when child refs collide.
func TestScrapeStaleness(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		t.Run(fmt.Sprintf("labelstore=%t", enabled), func(t *testing.T) {
			var removed atomic.Bool
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "text/plain; version=0.0.4")
				if !removed.Load() {
					fmt.Fprintln(w, `routed_test{tenant="a"} 1`)
				}
				fmt.Fprintln(w, `routed_test{tenant="b"} 2`)
			}))
			t.Cleanup(server.Close)

			samples := make(chan event, 1024)
			destination := func() storage.AppendableV2 {
				return prometheus.NewInterceptor(nil, prometheus.WithAppendHook(func(_ storage.SeriesRef, l labels.Labels, ts int64, v float64, _ storage.Appender) (storage.SeriesRef, error) {
					samples <- event{labels: l.Copy(), t: ts, v: v}
					return 42, nil
				}))
			}
			router, _, ls := newComponent(t, enabled, Arguments{
				Routes:           []Route{{Selector: `{tenant="a"}`, ForwardTo: destinations(destination())}},
				DefaultForwardTo: destinations(destination()),
			})

			var args scrape.Arguments
			args.SetToDefault()
			args.Targets = []discovery.Target{discovery.NewTargetFromMap(map[string]string{
				"__address__": strings.TrimPrefix(server.URL, "http://"),
			})}
			args.ForwardTo = destinations(router.receiver)
			args.ScrapeInterval = 100 * time.Millisecond
			args.ScrapeTimeout = 100 * time.Millisecond
			require.NoError(t, args.Validate())

			tc, err := componenttest.NewControllerFromID(util.TestLogger(t), "prometheus.scrape")
			require.NoError(t, err)
			ctx, cancel := context.WithCancel(t.Context())
			done := make(chan error, 1)
			go func() {
				done <- tc.Run(ctx, args, func(opts component.Options) component.Options {
					getServiceData := opts.GetServiceData
					opts.GetServiceData = func(name string) (any, error) {
						switch name {
						case labelstore.ServiceName:
							return ls, nil
						case httpservice.ServiceName:
							return httpservice.Data{
								HTTPListenAddr: "localhost:0", MemoryListenAddr: "alloy.internal:0",
								BaseHTTPPath: "/", DialFunc: (&net.Dialer{}).DialContext,
							}, nil
						case cluster.ServiceName:
							return cluster.Mock(), nil
						default:
							return getServiceData(name)
						}
					}
					return opts
				})
			}()
			t.Cleanup(func() {
				cancel()
				select {
				case err := <-done:
					require.NoError(t, err)
				case <-time.After(5 * time.Second):
					t.Error("scrape did not stop")
				}
			})
			require.NoError(t, tc.WaitRunning(5*time.Second))

			waitFor := func(match func(event) bool) {
				t.Helper()
				// The scrape manager can take one target-refresh interval to
				// start scraping; leave headroom on loaded CI workers.
				timer := time.NewTimer(10 * time.Second)
				defer timer.Stop()
				for {
					select {
					case sample := <-samples:
						if sample.labels.Get("__name__") == "routed_test" && match(sample) {
							return
						}
					case <-timer.C:
						t.Fatal("timed out waiting for routed scrape samples")
					}
				}
			}
			seen := map[string]bool{}
			waitFor(func(sample event) bool {
				seen[sample.labels.Get("tenant")] = true
				return seen["a"] && seen["b"]
			})
			removed.Store(true)
			waitFor(func(sample event) bool {
				if sample.labels.Get("tenant") == "b" {
					require.False(t, value.IsStaleNaN(sample.v), "the remaining series must not become stale")
					return false
				}
				return value.IsStaleNaN(sample.v)
			})
		})
	}
}
