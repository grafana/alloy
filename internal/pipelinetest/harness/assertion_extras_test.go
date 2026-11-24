package harness

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/prometheus/model/labels"
	"github.com/stretchr/testify/require"
)

func TestLogsContain(t *testing.T) {
	s := snapshot{logs: "level=warn msg=\"the `filetime` collector is deprecated\"\n"}

	require.NoError(t, LogsContain("is deprecated")(s))
	require.Error(t, LogsContain("is removed")(s))
}

func TestPrometheusSampleLabelRegex(t *testing.T) {
	matcher, err := PrometheusSampleLabelRegex("task", "/Microsoft/.*")
	require.NoError(t, err)

	s := snapshot{prometheus: []PrometheusSample{
		{Labels: labels.FromStrings("__name__", "windows_scheduled_task_state", "task", "/Microsoft/Windows/Defrag")},
		{Labels: labels.FromStrings("__name__", "windows_scheduled_task_state", "task", "/Probe")},
	}}

	require.NoError(t, PrometheusSampleCount(1, matcher)(s))
	require.Error(t, PrometheusSampleCount(0, matcher)(s))
}

func TestPrometheusSampleLabelNames(t *testing.T) {
	regex, err := PrometheusSampleLabelRegex("task", "/Microsoft/.*")
	require.NoError(t, err)
	// This is what an exact match of a literal __name__ and a task regex builds.
	matchers := []SampleMatcher{
		PrometheusSampleLabels(labels.FromStrings("__name__", "windows_scheduled_task_state"), true),
		regex,
		PrometheusSampleLabelNames([]string{"__name__", "task"}),
	}

	s := snapshot{prometheus: []PrometheusSample{
		{Labels: labels.FromStrings("__name__", "windows_scheduled_task_state", "task", "/Microsoft/Windows/Defrag")},
		{Labels: labels.FromStrings("__name__", "windows_scheduled_task_state", "state", "ready", "task", "/Microsoft/Windows/Defrag")},
		{Labels: labels.FromStrings("__name__", "windows_scheduled_task_state", "task", "/Probe")},
	}}
	require.NoError(t, PrometheusSampleCount(1, matchers...)(s))
	require.NoError(t, PrometheusContainsSample(matchers...)(s))
}

func TestPrometheusSampleLabelRegex_IsAnchored(t *testing.T) {
	matcher, err := PrometheusSampleLabelRegex("task", "/Microsoft")
	require.NoError(t, err)

	s := snapshot{prometheus: []PrometheusSample{
		{Labels: labels.FromStrings("task", "/Microsoft/Windows")},
	}}
	require.NoError(t, PrometheusSampleCount(0, matcher)(s))
}

func TestPrometheusSampleLabelRegex_RejectsBadPattern(t *testing.T) {
	_, err := PrometheusSampleLabelRegex("task", "(")
	require.Error(t, err)
}

func TestTailLogs(t *testing.T) {
	require.Equal(t, logTail{}, tailLogs(""))
	require.Equal(t, logTail{}, tailLogs(" \n\t\n"))

	short := tailLogs("one\ntwo\n")
	require.Equal(t, logTail{lines: []string{"one", "two"}}, short)
	require.Equal(t, "one\ntwo", short.String())

	var lines []string
	for i := 0; i < 100; i++ {
		lines = append(lines, fmt.Sprintf("line %d", i))
	}
	long := tailLogs(strings.Join(lines, "\n"))
	require.True(t, long.truncated)
	require.Equal(t, lines[100-maxRenderedLogLines:], long.lines)
	require.Equal(t, "...\n"+strings.Join(long.lines, "\n"), long.String())
}

func TestRenderSnapshot_OmitsLogsWhenEmpty(t *testing.T) {
	empty := renderSnapshot(snapshot{})
	require.Equal(t, empty+"\n\nhello", renderSnapshot(snapshot{logs: "hello\n"}))
}

func TestAssert_StableWindow(t *testing.T) {
	newAlloy := func(stableFor time.Duration) *Alloy {
		return &Alloy{sink: &Sink{}, logs: &logBuffer{}, assertTimeout: 5 * time.Second, stableFor: stableFor}
	}

	// Assertions that always pass succeed only after the window.
	start := time.Now()
	require.NoError(t, newAlloy(300*time.Millisecond).Assert(func(snapshot) error { return nil }))
	require.GreaterOrEqual(t, time.Since(start), 300*time.Millisecond)

	// A failure inside the window restarts the window.
	polls := 0
	start = time.Now()
	require.NoError(t, newAlloy(300*time.Millisecond).Assert(func(snapshot) error {
		polls++
		if polls == 3 {
			return AssertionError{Message: "late data"}
		}
		return nil
	}))
	require.GreaterOrEqual(t, time.Since(start), 300*time.Millisecond+2*defaultInterval)

	// A window longer than the timeout fails.
	a := newAlloy(time.Hour)
	a.assertTimeout = 200 * time.Millisecond
	require.Error(t, a.Assert(func(snapshot) error { return nil }))
}

func TestAssert_ReturnsEarlyWhenTheStableWindowCannotFit(t *testing.T) {
	newAlloy := func(timeout, stableFor time.Duration) *Alloy {
		return &Alloy{sink: &Sink{}, logs: &logBuffer{}, assertTimeout: timeout, stableFor: stableFor}
	}

	// Assertions pass at once, but the window is longer than the timeout.
	start := time.Now()
	err := newAlloy(5*time.Second, 10*time.Second).Assert(func(snapshot) error { return nil })
	require.ErrorContains(t, err, "cannot finish before the timeout")
	require.Less(t, time.Since(start), time.Second)

	// A failure with less than the window left ends the test, and the error
	// keeps the failing assertion.
	start = time.Now()
	err = newAlloy(2*time.Second, 5*time.Second).Assert(func(snapshot) error {
		return AssertionError{Kind: "x", Message: "never passes"}
	})
	require.ErrorContains(t, err, "never passes")
	require.ErrorContains(t, err, "not enough time left")
	require.Less(t, time.Since(start), time.Second)

	// A failure with enough time left keeps polling until the timeout.
	start = time.Now()
	err = newAlloy(400*time.Millisecond, 100*time.Millisecond).Assert(func(snapshot) error {
		return AssertionError{Kind: "x", Message: "never passes"}
	})
	require.ErrorContains(t, err, "never passes")
	require.GreaterOrEqual(t, time.Since(start), 250*time.Millisecond)
}

func TestAssert_StopsAtTheDeadline(t *testing.T) {
	// The default timeout is 30 seconds. The deadline is earlier, so it wins.
	a := &Alloy{sink: &Sink{}, logs: &logBuffer{}, assertTimeout: defaultTimeout, deadline: time.Now().Add(200 * time.Millisecond)}
	start := time.Now()
	err := a.Assert(func(snapshot) error { return AssertionError{Message: "never passes"} })
	require.ErrorContains(t, err, "never passes")
	require.Less(t, time.Since(start), 2*time.Second)
}
