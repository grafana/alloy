package enrich

import (
	"fmt"
	"sync"
	"testing"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/component/prometheus"
	"github.com/grafana/alloy/internal/service/labelstore"
	"github.com/grafana/alloy/internal/util"
	"github.com/grafana/alloy/internal/util/testappender"
	prom "github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/prometheus/model/histogram"
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
			name: "label enrichment with target_labels and differing label names",
			args: Arguments{
				Targets: []discovery.Target{
					discovery.NewTargetFromMap(map[string]string{
						"service": "test-service",
						"env":     "prod",
						"owner":   "team-a",
						"foo":     "bar",
					}),
				},
				TargetToMetricMatch: map[string]string{"service": "service_name"},
				LabelsToCopy:        []string{"env", "owner"},
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
				TargetToMetricMatch: map[string]string{"service": "service_name"},
				LabelsToCopy:        []string{"env", "owner"},
			},
			inputLabels: map[string]string{
				"service_name": "test-service",
			},
			expectedLabels: map[string]string{
				"service_name": "test-service",
			},
		},
		{
			name: "match when the label name is the same in targets and metrics",
			args: Arguments{
				Targets: []discovery.Target{
					discovery.NewTargetFromMap(map[string]string{
						"service": "test-service",
						"env":     "prod",
						"owner":   "team-a",
						"foo":     "bar",
					}),
				},
				TargetToMetricMatch: map[string]string{"service": "service"},
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
				Targets:             []discovery.Target{},
				TargetToMetricMatch: map[string]string{"service": "service"},
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
		for _, version := range testappender.Versions {
			t.Run(tt.name+"/"+version, func(t *testing.T) {
				var calls int
				fanout := terminalInterceptor(func(l labels.Labels) {
					calls++
					for name, value := range tt.expectedLabels {
						require.Equal(t, value, l.Get(name), "label %s", name)
					}
				})

				dest := testappender.NewCollectingAppender()
				var entry storage.AppendableV2
				tt.args.ForwardTo = []storage.AppendableV2{fanout, testappender.ConstantAppendable{Inner: dest}}
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

				// The samples, and their ST zeros, carry the enriched labels.
				lbls := labels.FromMap(tt.inputLabels)
				h := &histogram.Histogram{Count: 1, Sum: 1}
				appends := testappender.Appends{
					V1: func(app storage.Appender) error {
						if _, err := app.AppendSTZeroSample(0, lbls, 20, 10); err != nil {
							return err
						}
						if _, err := app.Append(0, lbls, 20, 1); err != nil {
							return err
						}
						if _, err := app.AppendHistogramSTZeroSample(0, lbls, 20, 10, h, nil); err != nil {
							return err
						}
						_, err := app.AppendHistogram(0, lbls, 20, h, nil)
						return err
					},
					V2: func(app storage.AppenderV2) error {
						if _, err := app.Append(0, lbls, 10, 20, 1, nil, nil, storage.AppendV2Options{}); err != nil {
							return err
						}
						_, err := app.Append(0, lbls, 10, 20, 0, h, nil, storage.AppendV2Options{})
						return err
					},
				}
				require.NoError(t, appends.AppendAndCommit(t.Context(), version, entry))
				require.Equal(t, 2, calls, "the samples didn't reach the next component")

				want := labels.FromMap(tt.expectedLabels).String()
				require.Contains(t, dest.CollectedSTZeroSamples(), want)
				require.Contains(t, dest.CollectedSTZeroHistograms(), want)
			})
		}
	}
}

func TestEnricherErrorsOnceExited(t *testing.T) {
	for _, version := range testappender.Versions {
		t.Run(version, func(t *testing.T) {
			c, err := New(component.Options{
				ID:             "1",
				OnStateChange:  func(component.Exports) {},
				Registerer:     prom.NewRegistry(),
				GetServiceData: getServiceData,
			}, Arguments{ForwardTo: []storage.AppendableV2{terminalInterceptor(func(labels.Labels) {})}})
			require.NoError(t, err)
			c.exited.Store(true)

			err = appendSample(labels.FromStrings("pod", "nginx-abc")).AppendAndCommit(t.Context(), version, c.receiver)
			require.ErrorContains(t, err, "has exited")
		})
	}
}

// appendSample appends a sample with the labels l.
func appendSample(l labels.Labels) testappender.Appends {
	return testappender.Appends{
		V1: func(app storage.Appender) error {
			_, err := app.Append(0, l, 20, 1)
			return err
		},
		V2: func(app storage.AppenderV2) error {
			_, err := app.Append(0, l, 0, 20, 1, nil, nil, storage.AppendV2Options{})
			return err
		},
	}
}

// terminalInterceptor ends the pipeline, and calls onAppend with the labels of
// every sample and histogram it gets through either appender version.
//
// TODO: Remove the V1 hooks when the V1 appenders are removed.
func terminalInterceptor(onAppend func(labels.Labels)) *prometheus.Interceptor {
	return prometheus.NewInterceptor(nil,
		prometheus.WithAppendV2Hook(func(ref storage.SeriesRef, l labels.Labels, _, _ int64, _ float64, _ *histogram.Histogram, _ *histogram.FloatHistogram, _ storage.AppendV2Options, _ storage.AppenderV2) (storage.SeriesRef, error) {
			onAppend(l)
			return ref, nil
		}),
		prometheus.WithAppendHook(func(ref storage.SeriesRef, l labels.Labels, _ int64, _ float64, _ storage.Appender) (storage.SeriesRef, error) {
			onAppend(l)
			return ref, nil
		}),
		prometheus.WithHistogramHook(func(ref storage.SeriesRef, l labels.Labels, _ int64, _ *histogram.Histogram, _ *histogram.FloatHistogram, _ storage.Appender) (storage.SeriesRef, error) {
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
			name: "valid new match",
			args: Arguments{
				TargetToMetricMatch: map[string]string{"ns": "namespace", "pod_name": "pod"},
			},
		},
		{
			name:    "error: no match mechanism",
			args:    Arguments{},
			wantErr: "target_to_metric_match must contain at least one label pair",
		},
		{
			name: "error: empty target label",
			args: Arguments{
				TargetToMetricMatch: map[string]string{"": "namespace"},
			},
			wantErr: "target_to_metric_match must not contain empty label names",
		},
		{
			name: "error: empty metric label",
			args: Arguments{
				TargetToMetricMatch: map[string]string{"namespace": ""},
			},
			wantErr: "target_to_metric_match must not contain empty label names",
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
	fanout := terminalInterceptor(func(labels.Labels) {})

	args := Arguments{
		Targets: []discovery.Target{
			discovery.NewTargetFromMap(map[string]string{"service": "svc", "env": "prod"}),
		},
		TargetToMetricMatch: map[string]string{"service": "service"},
		LabelsToCopy:        []string{"env"},
		ForwardTo:           []storage.AppendableV2{fanout},
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
	for _, version := range testappender.Versions {
		wg.Go(func() {
			for range iterations {
				_ = appendSample(lbls).AppendAndCommit(t.Context(), version, c.receiver)
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
