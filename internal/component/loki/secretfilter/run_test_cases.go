// run_test_cases.go provides RunTestCases for use by secretfilter tests and the
// extend package. It lives in the secretfilter package so it can use
// secretfilter.Arguments/Exports and avoids testhelper importing secretfilter.

package secretfilter

import (
	"testing"
	"time"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/loki/secretfilter/testhelper"
	"github.com/grafana/alloy/internal/util"
	"github.com/grafana/alloy/syntax"
	"github.com/grafana/loki/pkg/push"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"
)

// TestCase is a single test case for RunTestCases (name, log line, expected redaction).
type TestCase struct {
	Name         string
	InputLog     string
	ShouldRedact bool
}

// RunTestCases runs all cases through a single component (one config load).
// It builds the component once and calls processEntry for each case—no controller or channels.
func RunTestCases(t *testing.T, config string, cases []TestCase) {
	t.Helper()
	var args Arguments
	require.NoError(t, syntax.Unmarshal([]byte(config), &args))
	args.ForwardTo = []loki.LogsReceiver{loki.NewLogsReceiver()}

	opts := component.Options{
		Logger:         util.TestLogger(t),
		OnStateChange:  func(component.Exports) {},
		GetServiceData: testhelper.GetServiceData,
		Registerer:     prometheus.NewRegistry(),
	}
	c, err := New(opts, args)
	require.NoError(t, err)

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			require.NotEmpty(t, tc.InputLog)
			entry := loki.Entry{Labels: model.LabelSet{}, Entry: push.Entry{Timestamp: time.Now(), Line: tc.InputLog}}
			got := c.processEntry(entry)
			if tc.ShouldRedact {
				require.NotEqual(t, tc.InputLog, got.Line, "Expected log to be redacted but it was not")
			} else {
				require.Equal(t, tc.InputLog, got.Line, "Expected log to remain unchanged but it was modified")
			}
		})
	}
}

// DefaultTestCases returns the standard 7 cases from testhelper as []TestCase.
func DefaultTestCases() []TestCase {
	out := make([]TestCase, 0, len(testhelper.DefaultCases))
	for _, c := range testhelper.DefaultCases {
		out = append(out, TestCase{Name: c.Name, InputLog: c.InputLog, ShouldRedact: c.ShouldRedact})
	}
	return out
}
