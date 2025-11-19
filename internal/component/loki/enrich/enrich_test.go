package enrich

import (
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
		name           string
		args           Arguments
		inputLog       *push.Entry
		inputLabels    model.LabelSet
		expectedLabels model.LabelSet
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
			inputLog: &push.Entry{
				Timestamp: time.Now(),
				Line:      "test log",
			},
			inputLabels: model.LabelSet{
				"service_name": "test-service",
			},
			// foo:bar is not added as it is not in the target labels.
			expectedLabels: model.LabelSet{
				"service_name": "test-service",
				"env":          "prod",
				"owner":        "team-a",
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
			inputLog: &push.Entry{
				Timestamp: time.Now(),
				Line:      "test log",
			},
			inputLabels: model.LabelSet{
				"service_name": "test-service",
				"foo":          "bar",
			},
			expectedLabels: model.LabelSet{
				"service_name": "test-service",
				"foo":          "bar",
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
			inputLog: &push.Entry{
				Timestamp: time.Now(),
				Line:      "test log",
			},
			inputLabels: model.LabelSet{
				"service": "test-service",
			},
			expectedLabels: model.LabelSet{
				"service": "test-service",
				"env":     "prod",
				"owner":   "team-b",
				"region":  "us-west",
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
			inputLog: &push.Entry{
				Timestamp: time.Now(),
				Line:      "test log",
			},
			inputLabels: model.LabelSet{
				"service":  "test-service", // matches target_match_label
				"original": "label",
			},
			expectedLabels: model.LabelSet{
				"service":  "test-service",
				"original": "label",
				"env":      "prod",
				"owner":    "team-c",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a channel to receive enriched logs
			receivedCh := make(chan loki.Entry, 1)
			receiver := loki.NewLogsReceiver()

			// Create the component
			tt.args.ForwardTo = []loki.LogsReceiver{receiver}
			comp, err := New(opts, tt.args)
			require.NoError(t, err)

			// Start a goroutine to forward logs to our test channel
			go func() {
				for entry := range receiver.Chan() {
					receivedCh <- entry
				}
			}()

			// Process a log entry
			err = comp.processLog(tt.inputLog, tt.inputLabels)
			require.NoError(t, err)

			// Verify the enriched log
			select {
			case received := <-receivedCh:
				require.Equal(t, tt.expectedLabels, received.Labels)
				require.Equal(t, tt.inputLog.Line, received.Entry.Line)
			case <-time.After(time.Second):
				t.Fatal("timeout waiting for log entry")
			}
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

func TestName(t *testing.T) {
	comp, err := New(component.Options{
		Logger:        log.NewNopLogger(),
		OnStateChange: func(e component.Exports) {},
	}, Arguments{})
	require.NoError(t, err)
	require.Equal(t, "loki.enrich", comp.Name())
}
