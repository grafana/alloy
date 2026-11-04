package route

import (
	"context"
	"fmt"
	"strconv"
	"testing"

	prom "github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/prometheus/model/labels"
	promrelabel "github.com/prometheus/prometheus/model/relabel"
	"github.com/prometheus/prometheus/storage"

	"github.com/grafana/alloy/internal/component"
	commonrelabel "github.com/grafana/alloy/internal/component/common/relabel"
	"github.com/grafana/alloy/internal/component/prometheus"
	"github.com/grafana/alloy/internal/component/prometheus/appenders"
	"github.com/grafana/alloy/internal/component/prometheus/relabel"
	"github.com/grafana/alloy/internal/service/labelstore"
	"github.com/grafana/alloy/internal/service/livedebugging"
	"github.com/grafana/alloy/internal/util"
)

// BenchmarkRouting compares label-based dispatch with equivalent fan-out and
// keep rules. It measures repeated series, so the relabel caches are warm.
func BenchmarkRouting(b *testing.B) {
	for _, count := range []int{2, 8, 32} {
		for _, implementation := range []string{"route", "fanout_relabel"} {
			b.Run(fmt.Sprintf("%s/routes=%d", implementation, count), func(b *testing.B) {
				var input storage.Appendable
				if implementation == "route" {
					args := Arguments{}
					for i := range count {
						args.Routes = append(args.Routes, Route{
							Selector:  fmt.Sprintf(`{tenant="%d"}`, i),
							ForwardTo: destinations(appenders.Noop{}),
						})
					}
					c, _, _ := newComponent(b, false, args)
					input = c.receiver
				} else {
					input = relabelFanout(b, count)
				}

				series := make([]labels.Labels, 256)
				for i := range series {
					series[i] = labels.FromStrings("__name__", "requests_total", "tenant", strconv.Itoa(i%count), "instance", strconv.Itoa(i))
				}
				b.ReportAllocs()
				for b.Loop() {
					app := input.Appender(b.Context())
					for _, l := range series {
						if _, err := app.Append(0, l, 100, 1); err != nil {
							b.Fatal(err)
						}
					}
					if err := app.Commit(); err != nil {
						b.Fatal(err)
					}
				}
			})
		}
	}
}

func relabelFanout(b *testing.B, count int) *prometheus.Fanout {
	logger := util.TestLogger(b)
	ls := labelstore.New(logger, prom.NewRegistry(), false)
	debug := livedebugging.NewLiveDebugging()
	children := make([]storage.AppendableV2, 0, count)
	for i := range count {
		var receiver storage.AppendableV2
		c, err := relabel.New(component.Options{
			ID:         fmt.Sprintf("prometheus.relabel.route_%d", i),
			Logger:     logger,
			Registerer: prom.NewRegistry(),
			OnStateChange: func(exports component.Exports) {
				receiver = exports.(relabel.Exports).Receiver
			},
			GetServiceData: func(name string) (any, error) {
				if name == labelstore.ServiceName {
					return ls, nil
				}
				return debug, nil
			},
		}, relabel.Arguments{
			CacheSize: 100_000,
			ForwardTo: destinations(appenders.Noop{}),
			MetricRelabelConfigs: []*commonrelabel.Config{{
				Action:       commonrelabel.Keep,
				SourceLabels: []string{"tenant"},
				Regex:        commonrelabel.Regexp(promrelabel.MustNewRegexp(strconv.Itoa(i))),
			}},
		})
		if err != nil {
			b.Fatal(err)
		}
		children = append(children, receiver)
		b.Cleanup(func() {
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			_ = c.Run(ctx)
		})
	}
	fanout := prometheus.NewFanout(children, "benchmark", prom.NewRegistry(), ls)
	b.Cleanup(fanout.Clear)
	return fanout
}
