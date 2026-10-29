package enrich

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/component/prometheus"
	"github.com/grafana/alloy/internal/service/labelstore"
	"github.com/grafana/alloy/internal/util"
	prom "github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnricher(t *testing.T) {
	tests := []struct {
		name           string
		args           Arguments
		inputLabels    map[string]string
		expectedLabels map[string]string
	}{
		{
			name: "label enrichment with target_labels and metrics_match_label",
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
			inputLabels: map[string]string{
				"service_name": "test-service",
			},
			expectedLabels: map[string]string{
				"service_name": "test-service",
				"env":          "prod",
				"owner":        "team-a",
			},
		},
		{
			name: "mismatch metrics and targets",
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
			inputLabels: map[string]string{
				"service_name": "test-service",
			},
			expectedLabels: map[string]string{
				"service_name": "test-service",
			},
		},
		{
			name: "match using target_match_label when metrics_match_label is not specified",
			args: Arguments{
				Targets: []discovery.Target{
					discovery.NewTargetFromMap(map[string]string{
						"service": "test-service",
						"env":     "prod",
						"owner":   "team-a",
						"foo":     "bar",
					}),
				},
				TargetMatchLabel: "service",
			},
			inputLabels: map[string]string{
				"service": "test-service",
			},
			expectedLabels: map[string]string{
				"service": "test-service",
				"env":     "prod",
				"owner":   "team-a",
				"foo":     "bar",
			},
		},
		{
			name: "copy as is, when targets is empty",
			args: Arguments{
				Targets:          []discovery.Target{},
				TargetMatchLabel: "service",
			},
			inputLabels: map[string]string{
				"service": "test-service",
			},
			expectedLabels: map[string]string{
				"service": "test-service",
			},
		},
		{
			name: "multi-label match on namespace + pod + container",
			args: Arguments{
				Targets: []discovery.Target{
					discovery.NewTargetFromMap(map[string]string{
						"__meta_kubernetes_namespace":          "default",
						"__meta_kubernetes_pod_name":           "nginx-abc",
						"__meta_kubernetes_pod_container_name": "nginx",
						"pod_ip":                               "10.0.0.1",
					}),
				},
				TargetToMetricMatch: map[string]string{
					"__meta_kubernetes_namespace":          "namespace",
					"__meta_kubernetes_pod_name":           "pod",
					"__meta_kubernetes_pod_container_name": "container",
				},
				LabelsToCopy: []string{"pod_ip"},
			},
			inputLabels: map[string]string{
				"namespace": "default",
				"pod":       "nginx-abc",
				"container": "nginx",
			},
			expectedLabels: map[string]string{
				"namespace": "default",
				"pod":       "nginx-abc",
				"container": "nginx",
				"pod_ip":    "10.0.0.1",
			},
		},
		{
			name: "multi-label match: no match when metric missing a label",
			args: Arguments{
				Targets: []discovery.Target{
					discovery.NewTargetFromMap(map[string]string{
						"__meta_kubernetes_namespace":          "default",
						"__meta_kubernetes_pod_name":           "nginx-abc",
						"__meta_kubernetes_pod_container_name": "nginx",
						"pod_ip":                               "10.0.0.1",
					}),
				},
				TargetToMetricMatch: map[string]string{
					"__meta_kubernetes_namespace":          "namespace",
					"__meta_kubernetes_pod_name":           "pod",
					"__meta_kubernetes_pod_container_name": "container",
				},
				LabelsToCopy: []string{"pod_ip"},
			},
			// Metric is missing "container", so no match — labels pass through unchanged.
			inputLabels: map[string]string{
				"namespace": "default",
				"pod":       "nginx-abc",
			},
			expectedLabels: map[string]string{
				"namespace": "default",
				"pod":       "nginx-abc",
			},
		},
	}
	for _, tt := range tests {
		for _, path := range appendPaths() {
			t.Run(tt.name+"/"+path.name, func(t *testing.T) {
				var calls int
				check := func(l labels.Labels) {
					calls++
					for name, value := range tt.expectedLabels {
						require.Equal(t, value, l.Get(name), "label %s", name)
					}
				}
				fanout := nextWithHooks(check)

				var entry storage.AppendableV2
				tt.args.ForwardTo = []storage.AppendableV2{fanout}
				_, err := New(component.Options{
					ID:     "1",
					Logger: util.TestAlloyLogger(t).Slog(),
					OnStateChange: func(e component.Exports) {
						newE := e.(Exports)
						entry = newE.Receiver
					},
					Registerer:     prom.NewRegistry(),
					GetServiceData: getServiceData,
				}, tt.args)
				require.NoError(t, err)

				require.NoError(t, path.append(t.Context(), entry, labels.FromMap(tt.inputLabels)))
				require.Equal(t, 1, calls, "the sample didn't reach the next component")
			})
		}
	}
}

// appendPath appends a single sample to a receiver through one appender version.
type appendPath struct {
	name   string
	append func(ctx context.Context, entry storage.AppendableV2, l labels.Labels) error
}

// appendPaths returns every way a sample can reach a receiver, so tests can
// run the same scenario through each of them.
func appendPaths() []appendPath {
	return []appendPath{
		{
			name: "V1",
			append: func(ctx context.Context, entry storage.AppendableV2, l labels.Labels) error {
				app := entry.(storage.Appendable).Appender(ctx)
				if _, err := app.Append(0, l, time.Now().UnixMilli(), 0); err != nil {
					return err
				}
				return app.Commit()
			},
		},
	}
}

// nextWithHooks returns an interceptor that terminates the pipeline, and
// calls onAppend with the labels of every sample it receives.
func nextWithHooks(onAppend func(labels.Labels)) *prometheus.Interceptor {
	return prometheus.NewInterceptor(nil,
		prometheus.WithAppendHook(func(ref storage.SeriesRef, l labels.Labels, _ int64, _ float64, _ storage.Appender) (storage.SeriesRef, error) {
			onAppend(l)
			return ref, nil
		}),
	)
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		args    Arguments
		wantErr string
	}{
		{
			name: "valid legacy",
			args: Arguments{
				TargetMatchLabel: "service",
			},
		},
		{
			name: "valid legacy with metrics_match_label",
			args: Arguments{
				TargetMatchLabel:  "service",
				MetricsMatchLabel: "svc",
			},
		},
		{
			name: "valid new match",
			args: Arguments{
				TargetToMetricMatch: map[string]string{"ns": "namespace", "pod_name": "pod"},
			},
		},
		{
			name:    "error: no match mechanism",
			args:    Arguments{},
			wantErr: "at least one match mechanism must be specified",
		},
		{
			name: "new takes precedence over legacy",
			args: Arguments{
				TargetMatchLabel:    "service",
				TargetToMetricMatch: map[string]string{"ns": "namespace"},
			},
		},
		{
			name: "error: metrics_match_label without target_match_label",
			args: Arguments{
				MetricsMatchLabel: "svc",
			},
			wantErr: "target_match_label must be set",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.args.Validate()
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// TestEnrichConcurrentUpdate exercises Append/Update concurrently to surface
// data races.
func TestEnrichConcurrentUpdate(t *testing.T) {
	fanout := nextWithHooks(func(labels.Labels) {})

	args := Arguments{
		Targets: []discovery.Target{
			discovery.NewTargetFromMap(map[string]string{"service": "svc", "env": "prod"}),
		},
		TargetMatchLabel: "service",
		LabelsToCopy:     []string{"env"},
		ForwardTo:        []storage.AppendableV2{fanout},
	}

	c, err := New(component.Options{
		ID:             "1",
		OnStateChange:  func(component.Exports) {},
		Registerer:     prom.NewRegistry(),
		GetServiceData: getServiceData,
	}, args)
	require.NoError(t, err)

	const iterations = 1000
	lbls := labels.FromStrings("service", "svc")

	var wg sync.WaitGroup

	// append samples through every appender version
	for _, path := range appendPaths() {
		wg.Go(func() {
			for range iterations {
				_ = path.append(t.Context(), c.receiver, lbls)
			}
		})
	}

	// continuously rotate Targets and LabelsToCopy.
	wg.Go(func() {
		for range iterations {
			assert.NoError(t, c.Update(args))
		}
	})
	wg.Wait()
}

func getServiceData(name string) (any, error) {
	switch name {
	case labelstore.ServiceName:
		return labelstore.New(nil, prom.DefaultRegisterer), nil
	default:
		return nil, fmt.Errorf("service not found %s", name)
	}
}
