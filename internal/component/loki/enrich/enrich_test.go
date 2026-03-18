package enrich

import (
	"context"
	"testing"
	"time"

	"github.com/go-kit/log"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/loki/pkg/push"
)

func TestEnricher(t *testing.T) {
	// Create basic component options
	opts := component.Options{
		Logger:        log.NewNopLogger(),
		OnStateChange: func(e component.Exports) {},
	}

	tests := []struct {
		name     string
		args     Arguments
		input    loki.Entry
		expected loki.Entry
	}{
		{
			name: "label enrichment with target_labels and logs_match_label",
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
				LogsMatchLabel:   "service_name",
				LabelsToCopy:     []string{"env", "owner"},
			},
			input: loki.Entry{
				Labels: model.LabelSet{
					"service_name": "test-service",
				},
				Entry: push.Entry{
					Timestamp: time.Now(),
					Line:      "test log",
				},
			},
			// foo:bar is not added as it is not in the target labels.
			expected: loki.Entry{
				Labels: model.LabelSet{
					"service_name": "test-service",
					"env":          "prod",
					"owner":        "team-a",
				},
				Entry: push.Entry{
					Line: "test log",
				},
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
				TargetMatchLabel: "service",
				LogsMatchLabel:   "service_name",
				LabelsToCopy:     []string{"env"},
			},
			input: loki.Entry{
				Labels: model.LabelSet{
					"service_name": "test-service",
					"foo":          "bar",
				},
				Entry: push.Entry{
					Timestamp: time.Now(),
					Line:      "test log",
				},
			},
			expected: loki.Entry{
				Labels: model.LabelSet{
					"service_name": "test-service",
					"foo":          "bar",
				},
				Entry: push.Entry{
					Line: "test log",
				},
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
				TargetMatchLabel: "service",
				// LogsMatchLabel intentionally omitted as 'service' label exists in both.
			},
			input: loki.Entry{
				Labels: model.LabelSet{
					"service": "test-service",
				},
				Entry: push.Entry{
					Timestamp: time.Now(),
					Line:      "test log",
				},
			},
			expected: loki.Entry{
				Labels: model.LabelSet{
					"service": "test-service",
					"env":     "prod",
					"owner":   "team-b",
					"region":  "us-west",
				},
				Entry: push.Entry{
					Line: "test log",
				},
			},
		},
		{
			name: "match using target_match_label when logs_match_label is not specified",
			args: Arguments{
				Targets: []discovery.Target{
					discovery.NewTargetFromMap(map[string]string{
						"service": "test-service",
						"env":     "prod",
						"owner":   "team-c",
					}),
				},
				TargetMatchLabel: "service",
				// LogsMatchLabel intentionally omitted as 'service' label exists in both.
				LabelsToCopy: []string{"env", "owner"},
			},
			input: loki.Entry{
				Labels: model.LabelSet{
					"service":  "test-service", // matches target_match_label
					"original": "label",
				},
				Entry: push.Entry{
					Timestamp: time.Now(),
					Line:      "test log",
				},
			},
			expected: loki.Entry{
				Labels: model.LabelSet{
					"service":  "test-service",
					"original": "label",
					"env":      "prod",
					"owner":    "team-c",
				},
				Entry: push.Entry{
					Line: "test log",
				},
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
			opts.OnStateChange = func(e component.Exports) {
				exports = e.(Exports)
			}
			comp, err := New(opts, tt.args)
			require.NoError(t, err)
			require.NotNil(t, exports.Receiver)

			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			go func() {
				_ = comp.Run(ctx)
			}()

			exports.Receiver.Chan() <- tt.input

			require.Eventually(t, func() bool {
				return len(collector.Received()) == 1
			}, time.Second, 10*time.Millisecond)

			received := collector.Received()[0]
			require.Equal(t, tt.expected.Labels, received.Labels)
			require.Equal(t, tt.expected.Line, received.Line)
		})
	}
}

func TestUpdate(t *testing.T) {
	comp, err := New(component.Options{
		Logger:        log.NewNopLogger(),
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
		TargetMatchLabel: "service",
		LogsMatchLabel:   "service_name",
		LabelsToCopy:     []string{"env"},
	})
	require.NoError(t, err)
}
