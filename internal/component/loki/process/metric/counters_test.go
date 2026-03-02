package metric

// NOTE: This code is copied from Promtail (07cbef92268aecc0f20d1791a6df390c2df5c072) with changes kept to the minimum.

import (
	"testing"
	"testing/synctest"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"
)

var (
	counterTestTrue  = true
	counterTestFalse = false
	counterTestVal   = "some val"
)

func Test_validateCounterConfig(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		config CounterConfig
		err    string
	}{
		{"invalid action",
			CounterConfig{
				Action:  "del",
				MaxIdle: 1 * time.Second,
			},
			"the 'action' counter field must be either 'inc' or 'add'",
		},
		{"invalid counter match all",
			CounterConfig{
				MatchAll: counterTestTrue,
				Value:    counterTestVal,
				Action:   "inc",
				MaxIdle:  1 * time.Second,
			},
			"a 'counter' metric supports either 'match_all' or a 'value', but not both",
		},
		{"invalid counter match bytes",
			CounterConfig{
				MatchAll:        counterTestFalse,
				CountEntryBytes: counterTestTrue,
				Action:          "inc",
				MaxIdle:         1 * time.Second,
			},
			"the 'count_entry_bytes' counter field must be specified along with match_all set to true or action set to 'add'",
		},
		{"invalid counter match bytes action",
			CounterConfig{
				MatchAll:        counterTestTrue,
				CountEntryBytes: counterTestTrue,
				Action:          "inc",
				MaxIdle:         1 * time.Second,
			},
			"the 'count_entry_bytes' counter field must be specified along with match_all set to true or action set to 'add'",
		},
		{"valid counter match bytes",
			CounterConfig{
				MatchAll:        counterTestTrue,
				CountEntryBytes: counterTestTrue,
				Action:          "add",
				MaxIdle:         1 * time.Second,
			},
			"",
		},
		{"valid",
			CounterConfig{
				Value:   counterTestVal,
				Action:  "inc",
				MaxIdle: 1 * time.Second,
			},
			"",
		},
		{"valid match all is false",
			CounterConfig{
				MatchAll: counterTestFalse,
				Value:    counterTestVal,
				Action:   "inc",
				MaxIdle:  1 * time.Second,
			},
			"",
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.config.Validate()

			if err == nil {
				if tt.err != "" {
					t.Fatalf("Metrics stage validation error, expected error to cointain %q, but got no error", tt.err)
				}
			} else {
				if tt.err == "" {
					t.Fatalf("Metrics stage validation error, expected no error, but got %q", err)
				}
				assert.Contains(t, err.Error(), tt.err)
			}
		})
	}
}

func TestCounterExpiration(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		cfg := &CounterConfig{
			Action:      "inc",
			Description: "HELP ME!!!!!",
			MaxIdle:     1 * time.Second,
		}

		cnt, err := NewCounters("test1", cfg)
		assert.Nil(t, err)

		// Create a label and increment the counter
		lbl1 := model.LabelSet{}
		lbl1["test"] = "i don't wanna make this a constant"
		cnt.With(lbl1).Inc()

		// Collect the metrics, should still find the metric in the map
		collect(cnt)
		assert.Contains(t, cnt.metrics, lbl1.Fingerprint())

		time.Sleep(1100 * time.Millisecond) // Wait just past our max idle of 1 sec

		//Add another counter with new label val
		lbl2 := model.LabelSet{}
		lbl2["test"] = "eat this linter"
		cnt.With(lbl2).Inc()

		// Collect the metrics, first counter should have expired and removed, second should still be present
		collect(cnt)
		assert.NotContains(t, cnt.metrics, lbl1.Fingerprint())
		assert.Contains(t, cnt.metrics, lbl2.Fingerprint())
	})
}

func collect(c prometheus.Collector) {
	done := make(chan struct{})
	collector := make(chan prometheus.Metric)

	go func() {
		defer close(done)
		c.Collect(collector)
	}()

	for {
		select {
		case <-collector:
		case <-done:
			return
		}
	}
}
