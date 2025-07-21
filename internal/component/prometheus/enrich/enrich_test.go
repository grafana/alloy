package enrich

import (
	"fmt"
	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/component/prometheus"
	"github.com/grafana/alloy/internal/service/labelstore"
	"github.com/grafana/alloy/internal/util"
	prom "github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/storage"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestEnricher(t *testing.T) {
	// Create basic component options
	tests := []struct {
		name           string
		args           Arguments
		expectedLabels map[string]string
	}{
		{
			name: "test1",
			args: Arguments{
				Targets: []discovery.Target{
					discovery.NewTargetFromMap(map[string]string{
						"service": "test-service",
						"env":     "prod",
						"owner":   "team-a",
						"foo":     "bar",
					}),
				},
				TargetMatchLabel:  "service",
				MetricsMatchLabel: "service_name",
				LabelsToCopy:      []string{"env", "owner"},
			},
			expectedLabels: map[string]string{
				"service_name": "test-service",
				"env":          "prod",
				"owner":        "team-a",
			},
		},
		{
			name: "test2",
			args: Arguments{
				Targets: []discovery.Target{
					discovery.NewTargetFromMap(map[string]string{
						"service": "unknown",
						"env":     "prod",
						"owner":   "team-a",
						"foo":     "bar",
					}),
				},
				TargetMatchLabel:  "service",
				MetricsMatchLabel: "service_name",
				LabelsToCopy:      []string{"env", "owner"},
			},
			expectedLabels: map[string]string{
				"service_name": "test-service",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ls := labelstore.New(nil, prom.DefaultRegisterer)
			fanout := prometheus.NewInterceptor(
				nil, ls,
				prometheus.WithAppendHook(func(ref storage.SeriesRef, l labels.Labels, _ int64, _ float64, _ storage.Appender) (storage.SeriesRef, error) {
					for name, value := range tt.expectedLabels {
						require.Equal(t, l.Get(name), value)
					}

					return ref, nil
				}))

			var entry storage.Appendable
			tt.args.ForwardTo = []storage.Appendable{fanout}
			_, err := New(component.Options{
				ID:     "1",
				Logger: util.TestAlloyLogger(t),
				OnStateChange: func(e component.Exports) {
					newE := e.(Exports)
					entry = newE.Receiver
				},
				Registerer:     prom.NewRegistry(),
				GetServiceData: getServiceData,
			}, tt.args)

			require.NoError(t, err)

			lbls := labels.FromStrings("service_name", "test-service")
			app := entry.Appender(t.Context())

			_, err = app.Append(0, lbls, time.Now().UnixMilli(), 0)
			require.NoError(t, err)

			err = app.Commit()
			require.NoError(t, err)
		})
	}
}

func getServiceData(name string) (interface{}, error) {
	switch name {
	case labelstore.ServiceName:
		return labelstore.New(nil, prom.DefaultRegisterer), nil
	default:
		return nil, fmt.Errorf("service not found %s", name)
	}
}
