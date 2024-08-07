package stages

import (
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/grafana/loki/v3/pkg/logproto"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/util"
)

func TestMultilineStageProcess(t *testing.T) {
	logger := util.TestAlloyLogger(t)
	mcfg := MultilineConfig{Expression: "^START", MaxWaitTime: 3 * time.Second}
	regex, err := validateMultilineConfig(&mcfg)
	require.NoError(t, err)

	stage := &multilineStage{
		cfg:    mcfg,
		logger: logger,
		regex:  regex,
	}

	out := processEntries(stage,
		simpleEntry("not a start line before 1", "label"),
		simpleEntry("not a start line before 2", "label"),
		simpleEntry("START line 1", "label"),
		simpleEntry("not a start line", "label"),
		simpleEntry("START line 2", "label"),
		simpleEntry("START line 3", "label"))

	require.Len(t, out, 5)
	require.Equal(t, "not a start line before 1", out[0].Line)
	require.Equal(t, "not a start line before 2", out[1].Line)
	require.Equal(t, "START line 1\nnot a start line", out[2].Line)
	require.Equal(t, "START line 2", out[3].Line)
	require.Equal(t, "START line 3", out[4].Line)
}

func TestMultilineStageMultiStreams(t *testing.T) {
	logger := util.TestAlloyLogger(t)
	mcfg := MultilineConfig{Expression: "^START", MaxWaitTime: 3 * time.Second}
	regex, err := validateMultilineConfig(&mcfg)
	require.NoError(t, err)

	stage := &multilineStage{
		cfg:    mcfg,
		logger: logger,
		regex:  regex,
	}

	out := processEntries(stage,
		simpleEntry("START line 1", "one"),
		simpleEntry("not a start line 1", "one"),
		simpleEntry("START line 1", "two"),
		simpleEntry("not a start line 2", "one"),
		simpleEntry("START line 2", "two"),
		simpleEntry("START line 2", "one"),
		simpleEntry("not a start line 1", "one"),
	)

	sort.Slice(out, func(l, r int) bool {
		return out[l].Timestamp.Before(out[r].Timestamp)
	})

	require.Len(t, out, 4)

	require.Equal(t, "START line 1\nnot a start line 1\nnot a start line 2", out[0].Line)
	require.Equal(t, model.LabelValue("one"), out[0].Labels["value"])

	require.Equal(t, "START line 1", out[1].Line)
	require.Equal(t, model.LabelValue("two"), out[1].Labels["value"])

	require.Equal(t, "START line 2", out[2].Line)
	require.Equal(t, model.LabelValue("two"), out[2].Labels["value"])

	require.Equal(t, "START line 2\nnot a start line 1", out[3].Line)
	require.Equal(t, model.LabelValue("one"), out[3].Labels["value"])
}

func TestMultilineStageMaxWaitTime(t *testing.T) {
	logger := util.TestAlloyLogger(t)
	mcfg := MultilineConfig{Expression: "^START", MaxWaitTime: 100 * time.Millisecond}
	regex, err := validateMultilineConfig(&mcfg)
	require.NoError(t, err)

	stage := &multilineStage{
		cfg:    mcfg,
		logger: logger,
		regex:  regex,
	}

	in := make(chan Entry, 2)
	out := stage.Run(in)

	// Accumulate result
	mu := new(sync.Mutex)
	var res []Entry
	go func() {
		for e := range out {
			mu.Lock()
			t.Logf("appending %s", e.Line)
			res = append(res, e)
			mu.Unlock()
		}
	}()

	// Write input with a delay
	go func() {
		in <- simpleEntry("START line", "label")

		// Trigger flush due to max wait timeout
		time.Sleep(150 * time.Millisecond)

		in <- simpleEntry("not a start line hitting timeout", "label")

		// Signal pipeline we are done.
		close(in)
	}()

	require.Eventually(t, func() bool { mu.Lock(); defer mu.Unlock(); return len(res) == 2 }, 2*time.Second, 200*time.Millisecond)
	require.Equal(t, "START line", res[0].Line)
	require.Equal(t, "not a start line hitting timeout", res[1].Line)
}

func simpleEntry(line, label string) Entry {
	// We're adding a small wait time here, because on Windows, timers have a
	// smaller resolution than on Linux. This can mess with the ordering of log
	// lines, making the test Flaky on Windows runners.
	time.Sleep(1 * time.Millisecond)
	return Entry{
		Extracted: map[string]interface{}{},
		Entry: loki.Entry{
			Labels: model.LabelSet{"value": model.LabelValue(label)},
			Entry: logproto.Entry{
				Timestamp: time.Now(),
				Line:      line,
			},
		},
	}
}
