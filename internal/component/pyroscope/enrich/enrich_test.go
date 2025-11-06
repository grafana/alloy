package enrich

import (
	"context"
	"testing"

	"github.com/go-kit/log"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/component/pyroscope"
)

func TestEnricher(t *testing.T) {
	// Create basic component options
	opts := component.Options{
		Logger:        log.NewNopLogger(),
		OnStateChange: func(e component.Exports) {},
		Registerer: prometheus.NewRegistry(),
	}

	tests := []struct {
		name           string
		args           Arguments
		inputLabels    labels.Labels
		expectedLabels labels.Labels
	}{
		{
			name: "label enrichment with target_labels and profiles_match_label",
			args: Arguments{
				Targets: []discovery.Target{
					discovery.NewTargetFromMap(map[string]string{
						"service": "test-service",
						"env":     "prod",
						"owner":   "team-a",
						"foo":     "bar",
					}),
				},
				TargetMatchLabel:   "service",
				ProfilesMatchLabel: "service_name",
				LabelsToCopy:       []string{"env", "owner"},
			},
			inputLabels: labels.FromStrings(
				"service_name", "test-service",
			),
			// foo:bar is not added as it is not in the target labels.
			expectedLabels: labels.FromStrings(
				"service_name", "test-service",
				"env", "prod",
				"owner", "team-a",
			),
		},
		{
			name: "no match found. Copy profiles as is.",
			args: Arguments{
				Targets: []discovery.Target{
					discovery.NewTargetFromMap(map[string]string{
						"service": "different-service",
						"env":     "prod",
					}),
				},
				TargetMatchLabel:   "service",
				ProfilesMatchLabel: "service_name",
				LabelsToCopy:       []string{"env"},
			},
			inputLabels: labels.FromStrings(
				"service_name", "test-service",
				"foo", "bar",
			),
			expectedLabels: labels.FromStrings(
				"service_name", "test-service",
				"foo", "bar",
			),
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
				// ProfilesMatchLabel intentionally omitted as 'service' label exists in both.
			},
			inputLabels: labels.FromStrings(
				"service", "test-service",
			),
			expectedLabels: labels.FromStrings(
				"service", "test-service",
				"env", "prod",
				"owner", "team-b",
				"region", "us-west",
			),
		},
		{
			name: "match using target_match_label when profiles_match_label is not specified",
			args: Arguments{
				Targets: []discovery.Target{
					discovery.NewTargetFromMap(map[string]string{
						"service": "test-service",
						"env":     "prod",
						"owner":   "team-c",
					}),
				},
				TargetMatchLabel: "service",
				// ProfilesMatchLabel intentionally omitted as 'service' label exists in both.
				LabelsToCopy: []string{"env", "owner"},
			},
			inputLabels: labels.FromStrings(
				"service", "test-service", // matches target_match_label
				"original", "label",
			),
			expectedLabels: labels.FromStrings(
				"service", "test-service",
				"original", "label",
				"env", "prod",
				"owner", "team-c",
			),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var receivedLabels labels.Labels
			var receivedSamples []*pyroscope.RawSample

			// Synchronous appendable – enrichment occurs inline; no goroutines needed.
			testAppendable := pyroscope.AppendableFunc(func(_ context.Context, lbls labels.Labels, samples []*pyroscope.RawSample) error {
				receivedLabels = lbls
				receivedSamples = samples
				return nil
			})

			// Build component with forward target.
			tt.args.ForwardTo = []pyroscope.Appendable{testAppendable}
			comp, err := New(opts, tt.args)
			require.NoError(t, err)

			testSamples := []*pyroscope.RawSample{{
				ID:         "test-id",
				RawProfile: []byte("test profile data"),
			}}

			// Invoke Append synchronously.
			err = comp.exports.Receiver.Appender().Append(context.Background(), tt.inputLabels, testSamples)
			require.NoError(t, err)

			// Assert results directly.
			require.Equal(t, tt.expectedLabels, receivedLabels)
			require.Equal(t, testSamples, receivedSamples)
		})
	}
}

func TestUpdate(t *testing.T) {
	testAppendable := pyroscope.AppendableFunc(func(ctx context.Context, lbls labels.Labels, samples []*pyroscope.RawSample) error {
		return nil
	})

	comp, err := New(component.Options{
		Logger:        log.NewNopLogger(),
		OnStateChange: func(e component.Exports) {},
		Registerer: prometheus.NewRegistry(),
	}, Arguments{
		ForwardTo: []pyroscope.Appendable{testAppendable},
	})
	require.NoError(t, err)

	// Test updating targets
	newTargets := []discovery.Target{
		discovery.NewTargetFromMap(map[string]string{
			"service": "new-service",
			"env":     "prod",
		}),
	}

	err = comp.Update(Arguments{
		Targets:            newTargets,
		TargetMatchLabel:   "service",
		ProfilesMatchLabel: "service_name",
		LabelsToCopy:       []string{"env"},
		ForwardTo:          []pyroscope.Appendable{testAppendable},
	})
	require.NoError(t, err)
}

func TestName(t *testing.T) {
	testAppendable := pyroscope.AppendableFunc(func(ctx context.Context, lbls labels.Labels, samples []*pyroscope.RawSample) error {
		return nil
	})

	comp, err := New(component.Options{
		Logger:        log.NewNopLogger(),
		OnStateChange: func(e component.Exports) {},
		Registerer: prometheus.NewRegistry(),
	}, Arguments{
		ForwardTo: []pyroscope.Appendable{testAppendable},
	})
	require.NoError(t, err)
	require.Equal(t, "pyroscope.enrich", comp.Name())
}
