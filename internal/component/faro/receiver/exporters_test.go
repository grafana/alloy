package receiver

import (
	"strings"
	"testing"
	"time"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/faro/receiver/internal/payload"
	"github.com/grafana/alloy/internal/runtime/componenttest"
	"github.com/grafana/alloy/internal/util"
	"github.com/grafana/loki/pkg/push"
	"github.com/prometheus/client_golang/prometheus"
	promtestutil "github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"
)

var metricNames = []string{
	"faro_receiver_logs_total",
	"faro_receiver_measurements_total",
	"faro_receiver_exceptions_total",
	"faro_receiver_events_total",
}

func Test_metricsExporter_Export(t *testing.T) {
	var (
		reg = prometheus.NewRegistry()
		exp = newMetricsExporter(reg)
	)

	expect := `
		# HELP faro_receiver_logs_total Total number of ingested logs
		# TYPE faro_receiver_logs_total counter
		faro_receiver_logs_total 2

		# HELP faro_receiver_measurements_total Total number of ingested measurements
		# TYPE faro_receiver_measurements_total counter
		faro_receiver_measurements_total 3

		# HELP faro_receiver_exceptions_total Total number of ingested exceptions
		# TYPE faro_receiver_exceptions_total counter
		faro_receiver_exceptions_total 4

		# HELP faro_receiver_events_total Total number of ingested events
		# TYPE faro_receiver_events_total counter
		faro_receiver_events_total 5
	`

	p := payload.Payload{
		Logs:         make([]payload.Log, 2),
		Measurements: make([]payload.Measurement, 3),
		Exceptions:   make([]payload.Exception, 4),
		Events:       make([]payload.Event, 5),
	}
	require.NoError(t, exp.Export(t.Context(), p))

	err := promtestutil.CollectAndCompare(reg, strings.NewReader(expect), metricNames...)
	require.NoError(t, err)
}

func Test_LogsExporter_Export(t *testing.T) {
	now, err := time.Parse("2006-01-02T15:04:05Z0700", "2021-09-30T10:46:17.680Z")
	require.NoError(t, err)
	tt := []struct {
		desc    string
		format  LogFormat
		payload payload.Payload
		expect  loki.Entry
	}{
		{
			desc:   "export logfmt for log payload",
			format: FormatLogfmt,
			payload: payload.Payload{
				Logs: []payload.Log{
					{
						Message:   "React Router Future Flag Warning",
						LogLevel:  payload.LogLevelInfo,
						Timestamp: now,
						Trace: payload.TraceContext{
							TraceID: "a363d4f4417aa83158c437febd2d8838",
							SpanID:  "42876ecc4c1feafa",
						},
					},
				},
			},
			expect: loki.Entry{
				Labels: model.LabelSet{
					"foo":  model.LabelValue("bar"),
					"kind": model.LabelValue("log"),
				},
				Entry: push.Entry{
					Line: `timestamp="2021-09-30 10:46:17.68 +0000 UTC" kind=log message="React Router Future Flag Warning" level=info traceID=a363d4f4417aa83158c437febd2d8838 spanID=42876ecc4c1feafa browser_mobile=false`,
				},
			},
		},
		{
			desc:   "export logfmt for exception payload",
			format: FormatLogfmt,
			payload: payload.Payload{
				Exceptions: []payload.Exception{
					{
						Type:      "Error",
						Value:     "Cannot read property 'find' of undefined",
						Timestamp: now,
						Stacktrace: &payload.Stacktrace{
							Frames: []payload.Frame{
								{
									Function: "?",
									Filename: "http://fe:3002/static/js/vendors~main.chunk.js",
									Lineno:   8639,
									Colno:    42,
								},
								{
									Function: "dispatchAction",
									Filename: "http://fe:3002/static/js/vendors~main.chunk.js",
									Lineno:   268095,
									Colno:    9,
								},
							},
						},
					},
				},
			},
			expect: loki.Entry{
				Labels: model.LabelSet{
					"foo":  model.LabelValue("bar"),
					"kind": model.LabelValue("exception"),
				},
				Entry: push.Entry{
					Line: `timestamp="2021-09-30 10:46:17.68 +0000 UTC" kind=exception type=Error value="Cannot read property 'find' of undefined" stacktrace="Error: Cannot read property 'find' of undefined\n  at ? (http://fe:3002/static/js/vendors~main.chunk.js:8639:42)\n  at dispatchAction (http://fe:3002/static/js/vendors~main.chunk.js:268095:9)" hash=2735541995122471342 browser_mobile=false`,
				},
			},
		},
		{
			desc:   "export logfmt for measurement payload",
			format: FormatLogfmt,
			payload: payload.Payload{
				Measurements: []payload.Measurement{
					{
						Type: "sum",
						Values: map[string]float64{
							"ttfp":  20.12,
							"ttfcp": 22.12,
							"ttfb":  14,
						},
						Timestamp: now,
						Trace: payload.TraceContext{
							TraceID: "a363d4f4417aa83158c437febd2d8838",
							SpanID:  "42876ecc4c1feafa",
						},
						Context: payload.MeasurementContext{"host": "localhost"},
					},
				},
			},
			expect: loki.Entry{
				Labels: model.LabelSet{
					"foo":  model.LabelValue("bar"),
					"kind": model.LabelValue("measurement"),
				},
				Entry: push.Entry{
					Line: `timestamp="2021-09-30 10:46:17.68 +0000 UTC" kind=measurement type=sum ttfb=14.000000 ttfcp=22.120000 ttfp=20.120000 traceID=a363d4f4417aa83158c437febd2d8838 spanID=42876ecc4c1feafa context_host=localhost value_ttfb=14 value_ttfcp=22.12 value_ttfp=20.12 browser_mobile=false`,
				},
			},
		},
		{
			desc:   "export logfmt for event payload",
			format: FormatLogfmt,
			payload: payload.Payload{
				Events: []payload.Event{
					{
						Name:       "click_login_button",
						Domain:     "frontend",
						Attributes: map[string]string{"button_name": "login"},
						Timestamp:  now,
						Trace: payload.TraceContext{
							TraceID: "a363d4f4417aa83158c437febd2d8838",
							SpanID:  "42876ecc4c1feafa",
						},
					},
				},
			},
			expect: loki.Entry{
				Labels: model.LabelSet{
					"foo":  model.LabelValue("bar"),
					"kind": model.LabelValue("event"),
				},
				Entry: push.Entry{
					Line: `timestamp="2021-09-30 10:46:17.68 +0000 UTC" kind=event event_name=click_login_button event_domain=frontend event_data_button_name=login traceID=a363d4f4417aa83158c437febd2d8838 spanID=42876ecc4c1feafa browser_mobile=false`,
				},
			},
		},
		{
			desc:   "export json for log payload",
			format: FormatJSON,
			payload: payload.Payload{
				Logs: []payload.Log{
					{
						Message:   "React Router Future Flag Warning",
						LogLevel:  payload.LogLevelInfo,
						Timestamp: now,
						Trace: payload.TraceContext{
							TraceID: "a363d4f4417aa83158c437febd2d8838",
							SpanID:  "42876ecc4c1feafa",
						},
					},
				},
			},
			expect: loki.Entry{
				Labels: model.LabelSet{
					"foo":  model.LabelValue("bar"),
					"kind": model.LabelValue("log"),
				},
				Entry: push.Entry{
					Line: `{"browser_mobile":"false","kind":"log","level":"info","message":"React Router Future Flag Warning","spanID":"42876ecc4c1feafa","timestamp":"2021-09-30 10:46:17.68 +0000 UTC","traceID":"a363d4f4417aa83158c437febd2d8838"}`,
				},
			},
		},
		{
			desc:   "export json for exception payload",
			format: FormatJSON,
			payload: payload.Payload{
				Exceptions: []payload.Exception{
					{
						Type:      "Error",
						Value:     "Cannot read property 'find' of undefined",
						Timestamp: now,
						Stacktrace: &payload.Stacktrace{
							Frames: []payload.Frame{
								{
									Function: "?",
									Filename: "http://fe:3002/static/js/vendors~main.chunk.js",
									Lineno:   8639,
									Colno:    42,
								},
								{
									Function: "dispatchAction",
									Filename: "http://fe:3002/static/js/vendors~main.chunk.js",
									Lineno:   268095,
									Colno:    9,
								},
							},
						},
					},
				},
			},
			expect: loki.Entry{
				Labels: model.LabelSet{
					"foo":  model.LabelValue("bar"),
					"kind": model.LabelValue("exception"),
				},
				Entry: push.Entry{
					Line: `{"browser_mobile":"false","hash":"2735541995122471342","kind":"exception","stacktrace":"Error: Cannot read property 'find' of undefined\n  at ? (http://fe:3002/static/js/vendors~main.chunk.js:8639:42)\n  at dispatchAction (http://fe:3002/static/js/vendors~main.chunk.js:268095:9)","timestamp":"2021-09-30 10:46:17.68 +0000 UTC","type":"Error","value":"Cannot read property 'find' of undefined"}`,
				},
			},
		},
		{
			desc:   "export json for measurement payload",
			format: FormatJSON,
			payload: payload.Payload{
				Measurements: []payload.Measurement{
					{
						Type: "sum",
						Values: map[string]float64{
							"ttfp":  20.12,
							"ttfcp": 22.12,
							"ttfb":  14,
						},
						Timestamp: now,
						Trace: payload.TraceContext{
							TraceID: "a363d4f4417aa83158c437febd2d8838",
							SpanID:  "42876ecc4c1feafa",
						},
						Context: payload.MeasurementContext{"host": "localhost"},
					},
				},
			},
			expect: loki.Entry{
				Labels: model.LabelSet{
					"foo":  model.LabelValue("bar"),
					"kind": model.LabelValue("measurement"),
				},
				Entry: push.Entry{
					Line: `{"browser_mobile":"false","context_host":"localhost","kind":"measurement","spanID":"42876ecc4c1feafa","timestamp":"2021-09-30 10:46:17.68 +0000 UTC","traceID":"a363d4f4417aa83158c437febd2d8838","ttfb":"14.000000","ttfcp":"22.120000","ttfp":"20.120000","type":"sum","value_ttfb":14,"value_ttfcp":22.12,"value_ttfp":20.12}`,
				},
			},
		},
		{
			desc:   "export json for event payload",
			format: FormatJSON,
			payload: payload.Payload{
				Events: []payload.Event{
					{
						Name:       "click_login_button",
						Domain:     "frontend",
						Attributes: map[string]string{"button_name": "login"},
						Timestamp:  now,
						Trace: payload.TraceContext{
							TraceID: "a363d4f4417aa83158c437febd2d8838",
							SpanID:  "42876ecc4c1feafa",
						},
					},
				},
			},
			expect: loki.Entry{
				Labels: model.LabelSet{
					"foo":  model.LabelValue("bar"),
					"kind": model.LabelValue("event"),
				},
				Entry: push.Entry{
					Line: `{"browser_mobile":"false","event_data_button_name":"login","event_domain":"frontend","event_name":"click_login_button","kind":"event","spanID":"42876ecc4c1feafa","timestamp":"2021-09-30 10:46:17.68 +0000 UTC","traceID":"a363d4f4417aa83158c437febd2d8838"}`,
				},
			},
		},
	}
	for _, tc := range tt {
		t.Run(tc.desc, func(t *testing.T) {
			t.Parallel()
			var (
				lr  = newFakeLogsReceiver(t)
				exp = newLogsExporter(util.TestLogger(t), &varSourceMapsStore{}, tc.format)
			)
			exp.SetReceivers([]loki.LogsReceiver{lr})
			exp.SetLabels(map[string]string{
				"foo":  "bar",
				"kind": "",
			})
			ctx := componenttest.TestContext(t)
			require.NoError(t, exp.Export(ctx, tc.payload))

			lr.wg.Wait() // Wait for the fakelogreceiver goroutine to process
			require.Len(t, lr.GetEntries(), 1)
			require.Equal(t, tc.expect, lr.entries[0])
		})
	}
}
