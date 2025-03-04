package enrich

import (
	"testing"
	"time"

	"github.com/go-kit/log"
	"github.com/grafana/loki/v3/pkg/logproto"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/discovery"
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
		inputLog       *logproto.Entry
		inputLabels    model.LabelSet
		expectedLabels model.LabelSet
	}{
		{
			name: "basic label enrichment",
			args: Arguments{
				Targets: []discovery.Target{
					discovery.NewTargetFromMap(map[string]string{
						"service": "test-service",
						"env":     "prod",
						"owner":   "team-a",
					}),
				},
				TargetMatchLabel: "service",
				LogsMatchLabel:   "service_name",
				TargetLabels:     []string{"env", "owner"},
			},
			inputLog: &logproto.Entry{
				Timestamp: time.Now(),
				Line:      "test log",
			},
			inputLabels: model.LabelSet{
				"service_name": "test-service",
				"original":     "label",
			},
			expectedLabels: model.LabelSet{
				"service_name": "test-service",
				"original":     "label",
				"env":          "prod",
				"owner":        "team-a",
			},
		},
		{
			name: "no match found",
			args: Arguments{
				Targets: []discovery.Target{
					discovery.NewTargetFromMap(map[string]string{
						"service": "different-service",
						"env":     "prod",
					}),
				},
				TargetMatchLabel: "service",
				LogsMatchLabel:   "service_name",
				TargetLabels:     []string{"env"},
			},
			inputLog: &logproto.Entry{
				Timestamp: time.Now(),
				Line:      "test log",
			},
			inputLabels: model.LabelSet{
				"service_name": "test-service",
				"original":     "label",
			},
			expectedLabels: model.LabelSet{
				"service_name": "test-service",
				"original":     "label",
			},
		},
		{
			name: "copy all labels when target_labels is empty",
			args: Arguments{
				Targets: []discovery.Target{
					discovery.NewTargetFromMap(map[string]string{
						"service": "test-service",
						"env":     "prod",
						"owner":   "team-a",
						"region":  "us-west",
					}),
				},
				TargetMatchLabel: "service",
				// TargetLabels intentionally empty
			},
			inputLog: &logproto.Entry{
				Timestamp: time.Now(),
				Line:      "test log",
			},
			inputLabels: model.LabelSet{
				"service_name": "test-service",
				"original":     "label",
			},
			expectedLabels: model.LabelSet{
				"service_name": "test-service",
				"original":     "label",
				"service":      "test-service",
				"env":          "prod",
				"owner":        "team-a",
				"region":       "us-west",
			},
		},
		{
			name: "match using target_match_label when logs_match_label is not specified",
			args: Arguments{
				Targets: []discovery.Target{
					discovery.NewTargetFromMap(map[string]string{
						"service": "test-service",
						"env":     "prod",
						"owner":   "team-a",
					}),
				},
				TargetMatchLabel: "service",
				// LogsMatchLabel intentionally omitted
				TargetLabels: []string{"env", "owner"},
			},
			inputLog: &logproto.Entry{
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
				"owner":    "team-a",
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
		TargetLabels:     []string{"env"},
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
