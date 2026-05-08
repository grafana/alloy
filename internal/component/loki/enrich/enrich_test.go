package enrich

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/go-kit/log"
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
				TargetMatchLabel: "service",
				LogsMatchLabel:   "service_name",
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
				TargetMatchLabel: "service",
				// LogsMatchLabel intentionally omitted as 'service' label exists in both.
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			collector := loki.NewCollectingHandler()
			defer collector.Stop()

			var exports Exports

			// Create the component
			tt.args.ForwardTo = []loki.LogsReceiver{collector.Receiver()}

			opts := component.Options{
				Logger:        log.NewNopLogger(),
				SLogger:       logging.NewSlogNop(),
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

func TestUpdate(t *testing.T) {
	comp, err := New(component.Options{
		Logger:        log.NewNopLogger(),
		SLogger:       logging.NewSlogNop(),
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
