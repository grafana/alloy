package payload

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func loadTestData(t *testing.T, file string) []byte {
	t.Helper()
	// Safe to disable, this is a test.
	// nolint:gosec
	content, err := os.ReadFile(filepath.Join("../../testdata", file))
	require.NoError(t, err, "expected to be able to read file")
	require.True(t, len(content) > 0)
	return content
}

func TestUnmarshalPayloadJSON(t *testing.T) {
	content := loadTestData(t, "payload.json")
	var payload Payload
	err := json.Unmarshal(content, &payload)
	require.NoError(t, err)

	now, err := time.Parse("2006-01-02T15:04:05Z0700", "2021-09-30T10:46:17.680Z")
	require.NoError(t, err)

	require.Equal(t, Meta{
		SDK: SDK{
			Name:    "grafana-frontend-agent",
			Version: "1.0.0",
		},
		App: App{
			Name:        "testapp",
			Release:     "0.8.2",
			Version:     "abcdefg",
			Environment: "production",
		},
		User: User{
			Username:   "domasx2",
			ID:         "123",
			Email:      "geralt@kaermorhen.org",
			Attributes: map[string]string{"foo": "bar"},
		},
		Session: Session{
			ID:         "abcd",
			Attributes: map[string]string{"time_elapsed": "100s"},
		},
		Page: Page{
			URL: "https://example.com/page",
		},
		Browser: Browser{
			Name:    "chrome",
			Version: "88.12.1",
			OS:      "linux",
			Mobile:  false,
		},
		View: View{
			Name: "foobar",
		},
	}, payload.Meta)

	require.Len(t, payload.Exceptions, 1)
	require.Len(t, payload.Exceptions[0].Stacktrace.Frames, 26)
	require.Equal(t, "Error", payload.Exceptions[0].Type)
	require.Equal(t, "Cannot read property 'find' of undefined", payload.Exceptions[0].Value)
	require.EqualValues(t, ExceptionContext{"ReactError": "Annoying Error", "component": "ReactErrorBoundary"}, payload.Exceptions[0].Context)

	require.Equal(t, []Log{
		{
			Message:  "opened pricing page",
			LogLevel: LogLevelInfo,
			Context: map[string]string{
				"component": "AppRoot",
				"page":      "Pricing",
			},
			Timestamp: now,
			Trace: TraceContext{
				TraceID: "abcd",
				SpanID:  "def",
			},
		},
		{
			Message:  "loading price list",
			LogLevel: LogLevelTrace,
			Context: map[string]string{
				"component": "AppRoot",
				"page":      "Pricing",
			},
			Timestamp: now,
			Trace: TraceContext{
				TraceID: "abcd",
				SpanID:  "ghj",
			},
		},
	}, payload.Logs)

	require.Equal(t, []Event{
		{
			Name:      "click_login_button",
			Domain:    "frontend",
			Timestamp: now,
			Attributes: map[string]string{
				"foo": "bar",
				"one": "two",
			},
			Trace: TraceContext{
				TraceID: "abcd",
				SpanID:  "def",
			},
		},
		{
			Name:      "click_reset_password_button",
			Timestamp: now,
		},
	}, payload.Events)

	require.Len(t, payload.Measurements, 1)

	require.Equal(t, []Measurement{
		{
			Type: "foobar",
			Values: map[string]float64{
				"ttfp":  20.12,
				"ttfcp": 22.12,
				"ttfb":  14,
			},
			Timestamp: now,
			Trace: TraceContext{
				TraceID: "abcd",
				SpanID:  "def",
			},
			Context: MeasurementContext{
				"hello": "world",
			},
		},
	}, payload.Measurements)
}
