package api

import (
	"bytes"
	"net/http"
	"testing"
	"time"

	"github.com/alecthomas/units"
	"github.com/grafana/loki/pkg/push"
	"github.com/prometheus/common/model"
	promrelabel "github.com/prometheus/prometheus/model/relabel"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component/common/loki/client"
	"github.com/grafana/alloy/internal/component/loki/source"
	lutil "github.com/grafana/alloy/internal/loki/util"
)

const maxMessageSize = int(100 * units.MiB)

func TestLokiRoute(t *testing.T) {
	t.Run("applies labels relabeling and incoming timestamps", func(t *testing.T) {
		route := newLokiRoute(pathLokiPush, maxMessageSize)
		req := newPushRequest(t, `{stream="stream1",dropme="label",__anotherdroplabel="dropme"}`, []push.Entry{
			{
				Timestamp: time.Unix(0, 0),
				Line:      "line0",
				StructuredMetadata: push.LabelsAdapter{
					{Name: "i", Value: "0"},
					{Name: "anotherMetaData", Value: "val"},
				},
			},
			{
				Timestamp: time.Unix(99, 0),
				Line:      "line99",
			},
		}, nil)

		entries, status, err := route.Logs(req, &source.LogsConfig{
			FixedLabels:          model.LabelSet{"pushserver": "pushserver1"},
			RelabelRules:         []*promrelabel.Config{{Action: promrelabel.LabelDrop, Regex: promrelabel.MustNewRegexp("dropme")}},
			UseIncomingTimestamp: true,
		})
		require.NoError(t, err)
		require.Equal(t, http.StatusNoContent, status)
		require.Len(t, entries, 2)

		assert.Equal(t, model.LabelSet{
			"pushserver": "pushserver1",
			"stream":     "stream1",
		}, entries[0].Labels)
		assert.Equal(t, push.LabelsAdapter{
			{Name: "i", Value: "0"},
			{Name: "anotherMetaData", Value: "val"},
		}, entries[0].StructuredMetadata)
		assert.Equal(t, time.Unix(99, 0).Unix(), entries[1].Timestamp.Unix())
	})

	t.Run("adds tenant id label from header", func(t *testing.T) {
		route := newLokiRoute(pathPush, maxMessageSize)
		req := newPushRequest(t, `{stream="stream1"}`, []push.Entry{{Line: "line"}}, map[string]string{
			"X-Scope-OrgID": "tenant1",
		})

		entries, status, err := route.Logs(req, &source.LogsConfig{
			FixedLabels: model.LabelSet{"pushserver": "pushserver1"},
		})
		require.NoError(t, err)
		require.Equal(t, http.StatusNoContent, status)
		require.Len(t, entries, 1)

		assert.Equal(t, model.LabelSet{
			"pushserver": "pushserver1",
			"stream":     "stream1",
			model.LabelName(client.ReservedLabelTenantID): "tenant1",
		}, entries[0].Labels)
	})

	t.Run("overrides timestamp when incoming timestamps are disabled", func(t *testing.T) {
		initialTime := time.Unix(10, 0)

		route := newLokiRoute(pathLokiPush, maxMessageSize)
		req := newPushRequest(t, `{stream="stream1"}`, []push.Entry{{
			Timestamp: initialTime,
			Line:      "line",
		}}, nil)

		start := time.Now()
		entries, status, err := route.Logs(req, &source.LogsConfig{
			UseIncomingTimestamp: false,
		})
		require.NoError(t, err)
		require.Equal(t, http.StatusNoContent, status)
		require.Len(t, entries, 1)
		require.NotEqual(t, initialTime.Unix(), entries[0].Timestamp.Unix())
		require.GreaterOrEqual(t, entries[0].Timestamp.Unix(), start.Unix())
	})

	t.Run("returns bad request when message exceeds max size", func(t *testing.T) {
		route := newLokiRoute(pathLokiPush, 1)
		req := newPushRequest(t, `{stream="stream1"}`, []push.Entry{{Line: "line"}}, nil)

		entries, status, err := route.Logs(req, &source.LogsConfig{})
		require.Error(t, err)
		require.Equal(t, http.StatusBadRequest, status)
		require.Empty(t, entries)
		assert.ErrorContains(t, err, "message size too large")
	})
}

func TestPlainTextRoute(t *testing.T) {
	t.Run("applies fixed labels and sets timestamps", func(t *testing.T) {
		route := newPlainTextRoute(pathRaw)
		req := newPlainTextRequest(t, "line0\nline1\n", nil)

		start := time.Now()
		entries, status, err := route.Logs(req, &source.LogsConfig{
			FixedLabels: model.LabelSet{
				"pushserver": "pushserver2",
				"keepme":     "label",
			},
		})
		require.NoError(t, err)
		require.Equal(t, http.StatusNoContent, status)
		require.Len(t, entries, 2)

		assert.Equal(t, model.LabelSet{
			"pushserver": "pushserver2",
			"keepme":     "label",
		}, entries[0].Labels)
		assert.Equal(t, "line0", entries[0].Line)
		assert.Equal(t, "line1", entries[1].Line)
		assert.GreaterOrEqual(t, entries[1].Timestamp.Unix(), start.Unix())
	})
}

func newPushRequest(t *testing.T, labels string, entries []push.Entry, headers map[string]string) *http.Request {
	t.Helper()

	body := bytes.Buffer{}
	err := lutil.SerializeProto(&body, &push.PushRequest{
		Streams: []push.Stream{{
			Labels:  labels,
			Entries: entries,
		}},
	}, lutil.RawSnappy)
	require.NoError(t, err)

	// NOTE: URL do no matter here.
	req, err := http.NewRequest(http.MethodPost, "", &body)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/x-protobuf")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	return req
}

func newPlainTextRequest(t *testing.T, body string, headers map[string]string) *http.Request {
	t.Helper()

	// NOTE: URL do no matter here.
	req, err := http.NewRequest(http.MethodPost, "", bytes.NewBufferString(body))
	require.NoError(t, err)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	return req
}
