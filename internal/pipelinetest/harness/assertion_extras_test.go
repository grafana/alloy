package harness

import (
	"testing"

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
