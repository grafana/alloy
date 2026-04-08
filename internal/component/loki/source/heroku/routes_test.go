package heroku

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/go-kit/log"
	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/model"
	promrelabel "github.com/prometheus/prometheus/model/relabel"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	alloy_relabel "github.com/grafana/alloy/internal/component/common/relabel"
	"github.com/grafana/alloy/internal/component/loki/source"
)

const (
	routerPayload         = "270 <158>1 2022-06-13T14:52:23.622778+00:00 host heroku router - at=info method=GET path=\"/\" host=cryptic-cliffs-27764.herokuapp.com request_id=59da6323-2bc4-4143-8677-cc66ccfb115f fwd=\"181.167.87.140\" dyno=web.1 connect=0ms service=3ms status=200 bytes=6979 protocol=https\n" // trufflehog:ignore
	expectedRouterLogLine = "at=info method=GET path=\"/\" host=cryptic-cliffs-27764.herokuapp.com request_id=59da6323-2bc4-4143-8677-cc66ccfb115f fwd=\"181.167.87.140\" dyno=web.1 connect=0ms service=3ms status=200 bytes=6979 protocol=https\n"                                                                  // trufflehog:ignore

	appPayload         = "140 <190>1 2022-06-13T14:52:23.621815+00:00 host app web.1 - [GIN] 2022/06/13 - 14:52:23 | 200 |    1.428101ms |  181.167.87.140 | GET      \"/\"\n"
	expectedAppLogLine = "[GIN] 2022/06/13 - 14:52:23 | 200 |    1.428101ms |  181.167.87.140 | GET      \"/\"\n"
)

func TestDrainRoute(t *testing.T) {
	expectedTs, err := time.Parse(time.RFC3339Nano, "2022-06-13T14:52:23.621815+00:00")
	require.NoError(t, err)

	type expectedEntry struct {
		labels    model.LabelSet
		line      string
		timestamp *time.Time
	}

	tests := []struct {
		name    string
		params  map[string][]string
		body    string
		headers map[string]string
		cfg     *source.LogsConfig
		want    []expectedEntry
	}{
		{
			name:   "applies fixed labels and relabeling",
			params: map[string][]string{},
			body:   routerPayload,
			cfg: &source.LogsConfig{
				FixedLabels:  model.LabelSet{"foo": "bar"},
				RelabelRules: alloy_relabel.ComponentToPromRelabelConfigs(rulesExport),
			},
			want: []expectedEntry{{
				labels: model.LabelSet{
					"foo":    "bar",
					"host":   "host",
					"app":    "heroku",
					"proc":   "router",
					"log_id": "-",
				},
				line: expectedRouterLogLine,
			}},
		},
		{
			name: "applies relabeling to query parameters",
			params: map[string][]string{
				"some_query_param": {"app_123", "app_456"},
			},
			body: appPayload,
			cfg: &source.LogsConfig{
				RelabelRules: []*promrelabel.Config{
					{
						SourceLabels:         model.LabelNames{"__heroku_drain_param_some_query_param"},
						TargetLabel:          "query_param",
						Replacement:          "$1",
						Action:               promrelabel.Replace,
						Regex:                promrelabel.MustNewRegexp("(.*)"),
						NameValidationScheme: model.LegacyValidation,
					},
				},
			},
			want: []expectedEntry{{
				labels: model.LabelSet{
					"query_param": "app_123,app_456",
				},
				line: expectedAppLogLine,
			}},
		},
		{
			name: "uses incoming timestamps when enabled",
			body: appPayload,
			cfg: &source.LogsConfig{
				UseIncomingTimestamp: true,
			},
			want: []expectedEntry{{
				labels:    model.LabelSet{},
				line:      expectedAppLogLine,
				timestamp: &expectedTs,
			}},
		},
		{
			name: "adds tenant id label from header",
			body: appPayload,
			headers: map[string]string{
				"X-Scope-OrgID": "42",
			},
			cfg: &source.LogsConfig{
				RelabelRules: []*promrelabel.Config{
					{
						SourceLabels:         model.LabelNames{reservedLabelTenantID},
						TargetLabel:          "tenant_id",
						Replacement:          "$1",
						Action:               promrelabel.Replace,
						Regex:                promrelabel.MustNewRegexp("(.*)"),
						NameValidationScheme: model.LegacyValidation,
					},
				},
			},
			want: []expectedEntry{{
				labels: model.LabelSet{
					model.LabelName(reservedLabelTenantID): "42",
					"tenant_id":                            "42",
				},
				line: expectedAppLogLine,
			}},
		},
		{
			name: "parses multiple drain messages from a single request",
			body: routerPayload + appPayload,
			cfg:  &source.LogsConfig{},
			want: []expectedEntry{
				{
					labels: model.LabelSet{},
					line:   expectedRouterLogLine,
				},
				{
					labels: model.LabelSet{},
					line:   expectedAppLogLine,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			route := newDrainRoute(log.NewNopLogger(), newMetrics(prometheus.NewRegistry()))
			req := newDrainRequest(t, tt.params, tt.body, tt.headers)

			entries, status, err := route.Logs(req, tt.cfg)
			require.NoError(t, err)
			require.Equal(t, http.StatusNoContent, status)
			require.Len(t, entries, len(tt.want))

			for i, expected := range tt.want {
				assert.Equal(t, expected.labels, entries[i].Labels)
				assert.Equal(t, expected.line, entries[i].Line)
				if expected.timestamp != nil {
					assert.Equal(t, *expected.timestamp, entries[i].Timestamp)
				}
			}
		})
	}
}

func newDrainRequest(t *testing.T, params map[string][]string, body string, headers map[string]string) *http.Request {
	t.Helper()

	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("http://example%s", pathDrain), strings.NewReader(body))
	require.NoError(t, err)

	drainToken := uuid.New().String()
	frameID := uuid.New().String()

	values := url.Values{}
	for name, queryParams := range params {
		for _, p := range queryParams {
			values.Add(name, p)
		}
	}
	req.URL.RawQuery = values.Encode()

	req.Header.Set("Content-Type", "application/heroku_drain-1")
	req.Header.Set("Logplex-Drain-Token", fmt.Sprintf("d.%s", drainToken))
	req.Header.Set("Logplex-Frame-Id", frameID)
	req.Header.Set("Logplex-Msg-Count", "1")
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	return req
}
