package receiver

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/phayes/freeport"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/otelcol"
	"github.com/grafana/alloy/internal/runtime/componenttest"
	"github.com/grafana/alloy/internal/util"
	"github.com/grafana/loki/pkg/push"
)

// Test performs an end-to-end test of the component.
func Test(t *testing.T) {
	tt := []struct {
		desc      string
		logFormat LogFormat
		expect    loki.Entry
	}{
		{
			desc:      "format logfmt",
			logFormat: FormatLogfmt,
			expect: loki.Entry{
				Labels: model.LabelSet{
					"foo":  model.LabelValue("bar"),
					"kind": model.LabelValue("log"),
				},
				Entry: push.Entry{
					Line: `timestamp="2021-01-01 00:00:00 +0000 UTC" kind=log message="hello, world" level=info context_env=dev traceID=0 spanID=0 browser_mobile=false`,
				},
			},
		},
		{
			desc:      "format json",
			logFormat: FormatJSON,
			expect: loki.Entry{
				Labels: model.LabelSet{
					"foo":  model.LabelValue("bar"),
					"kind": model.LabelValue("log"),
				},
				Entry: push.Entry{
					Line: `{"browser_mobile":"false","context_env":"dev","kind":"log","level":"info","message":"hello, world","spanID":"0","timestamp":"2021-01-01 00:00:00 +0000 UTC","traceID":"0"}`,
				},
			},
		},
	}
	for _, tc := range tt {
		t.Run(tc.desc, func(t *testing.T) {
			t.Parallel()

			ctx := componenttest.TestContext(t)

			ctrl, err := componenttest.NewControllerFromID(
				util.TestLogger(t),
				"faro.receiver",
			)
			require.NoError(t, err)

			freePort, err := freeport.GetFreePort()
			require.NoError(t, err)

			lr := newFakeLogsReceiver(t)

			go func() {
				err := ctrl.Run(ctx, Arguments{
					LogLabels: map[string]string{
						"foo":  "bar",
						"kind": "",
					},
					LogFormat: tc.logFormat,

					Server: ServerArguments{
						Host:            "127.0.0.1",
						Port:            freePort,
						IncludeMetadata: true,
					},

					Output: OutputArguments{
						Logs:   []loki.LogsReceiver{lr},
						Traces: []otelcol.Consumer{},
					},
				})
				require.NoError(t, err)
			}()

			// Wait for the server to be running.
			util.Eventually(t, func(t require.TestingT) {
				resp, err := http.Get(fmt.Sprintf("http://localhost:%d/-/ready", freePort))
				require.NoError(t, err)
				defer resp.Body.Close()

				require.Equal(t, http.StatusOK, resp.StatusCode)
			})

			// Send a sample payload to the server.
			req, err := http.NewRequest(
				"POST",
				fmt.Sprintf("http://localhost:%d/collect", freePort),
				strings.NewReader(`{
			"traces": {
				"resourceSpans": []
			},
			"logs": [{
				"message": "hello, world",
				"level": "info",
				"context": {"env": "dev"},
				"timestamp": "2021-01-01T00:00:00Z",
				"trace": {
					"trace_id": "0",
					"span_id": "0"
				}
			}],
			"exceptions": [],
			"measurements": [],
			"meta": {}
		}`),
			)
			require.NoError(t, err)

			req.Header.Add("Tenant-Id", "TENANTID")
			req.Header.Add("Content-Type", "application/json")

			client := &http.Client{}
			resp, err := client.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			require.Equal(t, http.StatusAccepted, resp.StatusCode)
			lr.wg.Wait() // Wait for the fakelogreceiver goroutine to process
			require.Len(t, lr.GetEntries(), 1)

			require.Equal(t, tc.expect, lr.entries[0])
		})
	}
}

type fakeLogsReceiver struct {
	loki.LogsReceiver
	ch chan loki.Entry

	entriesMut sync.RWMutex
	wg         sync.WaitGroup
	entries    []loki.Entry
}

var _ loki.LogsReceiver = (*fakeLogsReceiver)(nil)

func newFakeLogsReceiver(t *testing.T) *fakeLogsReceiver {
	ctx := componenttest.TestContext(t)

	lr := &fakeLogsReceiver{
		ch: make(chan loki.Entry, 1),
	}

	lr.wg.Add(1)
	go func() {
		defer close(lr.ch)
		defer lr.wg.Done()

		select {
		case <-ctx.Done():
			return
		case ent := <-lr.Chan():
			lr.entriesMut.Lock()
			lr.entries = append(lr.entries, loki.Entry{
				Labels: ent.Labels,
				Entry: push.Entry{
					Timestamp:          time.Time{}, // Use consistent time for testing.
					Line:               ent.Line,
					StructuredMetadata: ent.StructuredMetadata,
				},
			})
			lr.entriesMut.Unlock()
		}
	}()

	return lr
}

func (lr *fakeLogsReceiver) Chan() chan loki.Entry {
	return lr.ch
}

func (lr *fakeLogsReceiver) GetEntries() []loki.Entry {
	lr.entriesMut.RLock()
	defer lr.entriesMut.RUnlock()
	return lr.entries
}
