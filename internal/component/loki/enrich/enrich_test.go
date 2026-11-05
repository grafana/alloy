package enrich

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/grafana/loki/pkg/push"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/runtime/logging"
)

func TestEnricher(t *testing.T) {
	var (
		now        = time.Now()
		inputEntry = push.Entry{
			Timestamp: now,
			Line:      "test log",
		}
		expectedEntry = push.Entry{
			Line:      "test log",
			Timestamp: now,
		}
	)

	type testCase struct {
		name     string
		args     Arguments
		input    loki.Entry
		expected loki.Entry
	}

	tests := []testCase{
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
				TargetToLogMatch: map[string]string{"service": "service_name"},
				LabelsToCopy:     []string{"env", "owner"},
			},
			input: loki.Entry{
				Labels: model.LabelSet{
					"service_name": "test-service",
				},
				Entry: inputEntry,
			},
			// foo:bar is not added as it is not in the target labels.
			expected: loki.Entry{
				Labels: model.LabelSet{
					"service_name": "test-service",
					"env":          "prod",
					"owner":        "team-a",
				},
				Entry: expectedEntry,
			},
		},
		{
			name: "no match found. Copy logs as is.",
			args: Arguments{
				Targets: []discovery.Target{
					discovery.NewTargetFromMap(map[string]string{
						"service": "different-service",
						"env":     "prod",
					}),
				},
				TargetToLogMatch: map[string]string{"service": "service_name"},
				LabelsToCopy:     []string{"env"},
			},
			input: loki.Entry{
				Labels: model.LabelSet{
					"service_name": "test-service",
					"foo":          "bar",
				},
				Entry: inputEntry,
			},
			expected: loki.Entry{
				Labels: model.LabelSet{
					"service_name": "test-service",
					"foo":          "bar",
				},
				Entry: expectedEntry,
			},
		},
		{
			name: "copy all labels when target_labels is empty",
			args: Arguments{
				Targets: []discovery.Target{
					discovery.NewTargetFromMap(map[string]string{
						"service": "test-service",
						"env":     "prod",
						"owner":   "team-b",
						"region":  "us-west",
					}),
				},
				TargetToLogMatch: map[string]string{"service": "service"},
			},
			input: loki.Entry{
				Labels: model.LabelSet{
					"service": "test-service",
				},
				Entry: inputEntry,
			},
			expected: loki.Entry{
				Labels: model.LabelSet{
					"service": "test-service",
					"env":     "prod",
					"owner":   "team-b",
					"region":  "us-west",
				},
				Entry: expectedEntry,
			},
		},
		{
			name: "match when the label name is the same in targets and logs",
			args: Arguments{
				Targets: []discovery.Target{
					discovery.NewTargetFromMap(map[string]string{
						"service": "test-service",
						"env":     "prod",
						"owner":   "team-c",
					}),
				},
				TargetToLogMatch: map[string]string{"service": "service"},
				LabelsToCopy:     []string{"env", "owner"},
			},
			input: loki.Entry{
				Labels: model.LabelSet{
					"service":  "test-service", // matches the target_to_log_match label
					"original": "label",
				},
				Entry: inputEntry,
			},
			expected: loki.Entry{
				Labels: model.LabelSet{
					"service":  "test-service",
					"original": "label",
					"env":      "prod",
					"owner":    "team-c",
				},
				Entry: expectedEntry,
			},
		},
		{
			name: "multi-label match selects the target using all labels",
			args: Arguments{
				Targets: []discovery.Target{
					discovery.NewTargetFromMap(map[string]string{
						"__meta_kubernetes_namespace": "production",
						"__meta_kubernetes_pod_name":  "api-0",
						"environment":                 "prod",
					}),
					discovery.NewTargetFromMap(map[string]string{
						"__meta_kubernetes_namespace": "staging",
						"__meta_kubernetes_pod_name":  "api-0",
						"environment":                 "stage",
					}),
				},
				TargetToLogMatch: map[string]string{
					"__meta_kubernetes_namespace": "namespace",
					"__meta_kubernetes_pod_name":  "pod",
				},
				LabelsToCopy: []string{"environment"},
			},
			input: loki.Entry{
				Labels: model.LabelSet{
					"namespace": "staging",
					"pod":       "api-0",
				},
				Entry: inputEntry,
			},
			expected: loki.Entry{
				Labels: model.LabelSet{
					"namespace":   "staging",
					"pod":         "api-0",
					"environment": "stage",
				},
				Entry: expectedEntry,
			},
		},
		{
			name: "multi-label match requires every log label",
			args: Arguments{
				Targets: []discovery.Target{
					discovery.NewTargetFromMap(map[string]string{
						"__meta_kubernetes_namespace": "staging",
						"__meta_kubernetes_pod_name":  "api-0",
						"environment":                 "stage",
					}),
				},
				TargetToLogMatch: map[string]string{
					"__meta_kubernetes_namespace": "namespace",
					"__meta_kubernetes_pod_name":  "pod",
				},
				LabelsToCopy: []string{"environment"},
			},
			input: loki.Entry{
				Labels: model.LabelSet{
					"namespace": "staging",
				},
				Entry: inputEntry,
			},
			expected: loki.Entry{
				Labels: model.LabelSet{
					"namespace": "staging",
				},
				Entry: expectedEntry,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			collector := loki.NewCollectingHandler()
			defer collector.Stop()

			var exports Exports

			// Create the component
			tt.args.ForwardTo = []loki.LogsReceiver{collector.Receiver()}

			opts := component.Options{
				Logger:        logging.NewSlogNop(),
				OnStateChange: func(e component.Exports) {},
			}
			opts.OnStateChange = func(e component.Exports) {
				exports = e.(Exports)
			}
			comp, err := New(opts, tt.args)
			require.NoError(t, err)
			require.NotNil(t, exports.Receiver)

			ctx, cancel := context.WithCancel(t.Context())
			var wg sync.WaitGroup
			wg.Go(func() {
				_ = comp.Run(ctx)
			})

			exports.Receiver.Chan() <- tt.input

			require.Eventually(t, func() bool {
				return len(collector.Received()) == 1
			}, time.Second, 10*time.Millisecond)

			received := collector.Received()[0]
			require.Equal(t, tt.expected.Labels, received.Labels)
			require.Equal(t, tt.expected.Line, received.Line)

			cancel()
			wg.Wait()
		})
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		args    Arguments
		wantErr string
	}{
		{
			name: "valid multi-label match",
			args: Arguments{
				TargetToLogMatch: map[string]string{"namespace": "namespace", "pod_name": "pod"},
			},
		},
		{
			name:    "missing match mechanism",
			args:    Arguments{},
			wantErr: "target_to_log_match must contain at least one label pair",
		},
		{
			name: "empty target label in target_to_log_match",
			args: Arguments{
				TargetToLogMatch: map[string]string{"": "namespace"},
			},
			wantErr: "target_to_log_match must not contain empty label names",
		},
		{
			name: "empty log label in target_to_log_match",
			args: Arguments{
				TargetToLogMatch: map[string]string{"namespace": ""},
			},
			wantErr: "target_to_log_match must not contain empty label names",
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

func TestUpdate(t *testing.T) {
	comp, err := New(component.Options{
		Logger:        logging.NewSlogNop(),
		OnStateChange: func(e component.Exports) {},
	}, Arguments{})
	require.NoError(t, err)

	// Test updating targets
	newTargets := []discovery.Target{
		discovery.NewTargetFromMap(map[string]string{
			"service": "new-service",
			"env":     "prod",
		}),
	}

	err = comp.Update(Arguments{
		Targets:          newTargets,
		TargetToLogMatch: map[string]string{"service": "service_name"},
		LabelsToCopy:     []string{"env"},
	})
	require.NoError(t, err)
}
