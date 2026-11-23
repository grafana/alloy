package harness

import (
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

func TestRenderLogs(t *testing.T) {
	require.Equal(t, "logs: none", renderLogs(""))
	require.Contains(t, renderLogs("one\ntwo\n"), "two")

	var lines []string
	for i := 0; i < 100; i++ {
		lines = append(lines, "line")
	}
	lines[99] = "last"
	out := renderLogs(strings.Join(lines, "\n"))
	require.Contains(t, out, "last 40 lines")
	require.Contains(t, out, "last")
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
