package loki

import (
	"testing"
	"time"

	"github.com/grafana/loki/pkg/push"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"
)

func TestAddLabelsMiddleware(t *testing.T) {
	tests := []struct {
		name             string
		additionalLabels model.LabelSet
		inputEntry       Entry
		expectedLabels   model.LabelSet
	}{
		{
			name: "nil labels",
			additionalLabels: model.LabelSet{
				"service": "test-service",
				"env":     "production",
				"region":  "us-west-1",
			},
			inputEntry: Entry{
				Labels: nil,
				Entry:  testEntry(),
			},
			expectedLabels: model.LabelSet{
				"service": "test-service",
				"env":     "production",
				"region":  "us-west-1",
			},
		},
		{
			name:             "nil additional labels",
			additionalLabels: nil,
			inputEntry: Entry{
				Labels: model.LabelSet{
					"level": "info",
				},
				Entry: testEntry(),
			},
			expectedLabels: model.LabelSet{
				"level": "info",
			},
		},
		{
			name: "add multiple labels",
			additionalLabels: model.LabelSet{
				"service": "test-service",
				"env":     "production",
				"region":  "us-west-1",
			},
			inputEntry: Entry{
				Labels: model.LabelSet{
					"level": "error",
				},
				Entry: testEntry(),
			},
			expectedLabels: model.LabelSet{
				"level":   "error",
				"service": "test-service",
				"env":     "production",
				"region":  "us-west-1",
			},
		},
		{
			name: "try override existing label",
			additionalLabels: model.LabelSet{
				"level": "debug",
			},
			inputEntry: Entry{
				Labels: model.LabelSet{
					"level": "info",
				},
				Entry: testEntry(),
			},
			expectedLabels: model.LabelSet{
				"level": "info",
			},
		},
		{
			name: "set label to empty value",
			additionalLabels: model.LabelSet{
				"service": "",
			},
			inputEntry: Entry{
				Labels: model.LabelSet{
					"level":   "info",
					"service": "test-service",
				},
				Entry: testEntry(),
			},
			expectedLabels: model.LabelSet{
				"level":   "info",
				"service": "test-service",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a test entry handler that captures the received entry
			var receivedEntry Entry
			received := make(chan Entry, 1)
			testHandler := NewEntryHandler(received, func() {})

			// Create the middleware and wrap the test handler
			middleware := AddLabelsMiddleware(tt.additionalLabels)
			wrappedHandler := middleware.Wrap(testHandler)

			// Send the input entry
			wrappedHandler.Chan() <- tt.inputEntry
			wrappedHandler.Stop()

			// Check the received entry
			select {
			case receivedEntry = <-received:
				assert.Equal(t, tt.expectedLabels, receivedEntry.Labels)
			case <-time.After(time.Second):
				t.Fatal("timeout waiting for entry")
			}
		})
	}
}

func testEntry() push.Entry {
	return push.Entry{
		Timestamp: time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC),
		Line:      "test log line",
	}
}
