package aws_firehose

import (
	"bytes"
	"compress/gzip"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/relabel"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/loki/source"
)

const (
	testRequestID = "86208cf6-2bcc-47e6-9010-02ca9f44a025"
	testSourceARN = "arn:aws:firehose:us-east-2:123:deliverystream/aws_firehose_test_stream"

	directPutRequestTimestamp = 1684422829730
)

//go:embed testdata/*
var testData embed.FS

var cwLogsTimestamps = []int64{
	1684423980083,
	1684424003641,
	1684424003820,
	1684424003822,
	1684424003859,
	1684424003859,
	1684424005707,
	1684424005708,
	1684424005718,
	1684424005718,
	1684424007492,
	1684424007493,
	1684424007494,
	1684424007494,
}

func readTestData(t *testing.T, name string) string {
	t.Helper()

	f, err := testData.ReadFile(name)
	if err != nil {
		require.FailNow(t, fmt.Sprintf("error reading test data: %s", name))
	}
	return string(f)
}

func TestRoute(t *testing.T) {
	type testcase struct {
		Name          string
		TenantID      string
		UseIncomingTs bool
		Body          string
		Relabels      []*relabel.Config
		Assert        func(t *testing.T, res *httptest.ResponseRecorder, entries []loki.Entry)
		AssertMetrics func(t *testing.T, m []*dto.MetricFamily)
	}

	tests := []testcase{
		{
			Name: "direct put data",
			Body: readTestData(t, "testdata/direct_put.json"),
			Assert: func(t *testing.T, res *httptest.ResponseRecorder, entries []loki.Entry) {
				var r firehoseResponse
				require.NoError(t, json.Unmarshal(res.Body.Bytes(), &r))

				require.Equal(t, http.StatusOK, res.Code)
				require.Equal(t, testRequestID, r.RequestID)
				require.Len(t, entries, 3)
				for _, e := range entries {
					require.NotContains(t, e.Labels, "__tenant_id__")
				}
			},
		},
		{
			Name:     "direct put data, with tenant ID",
			Body:     readTestData(t, "testdata/direct_put.json"),
			TenantID: "20",
			Assert: func(t *testing.T, res *httptest.ResponseRecorder, entries []loki.Entry) {
				var r firehoseResponse
				require.NoError(t, json.Unmarshal(res.Body.Bytes(), &r))

				require.Equal(t, http.StatusOK, res.Code)
				require.Len(t, entries, 3)
				for _, e := range entries {
					require.Equal(t, "20", string(e.Labels["__tenant_id__"]))
				}
			},
		},
		{
			Name: "direct put data, relabeling req id and source arn",
			Body: readTestData(t, "testdata/direct_put.json"),
			Relabels: []*relabel.Config{
				keepLabelRule("__aws_firehose_request_id", "aws_request_id"),
				keepLabelRule("__aws_firehose_source_arn", "aws_source_arn"),
			},
			Assert: func(t *testing.T, res *httptest.ResponseRecorder, entries []loki.Entry) {
				var r firehoseResponse
				require.NoError(t, json.Unmarshal(res.Body.Bytes(), &r))

				require.Equal(t, http.StatusOK, res.Code)
				require.Equal(t, testRequestID, r.RequestID)
				require.Len(t, entries, 3)
				for _, e := range entries {
					require.Equal(t, testRequestID, string(e.Labels["aws_request_id"]))
					require.Equal(t, testSourceARN, string(e.Labels["aws_source_arn"]))
				}
			},
		},
		{
			Name: "direct put data with non JSON data",
			Body: readTestData(t, "testdata/direct_put_with_non_json_message.json"),
			Assert: func(t *testing.T, res *httptest.ResponseRecorder, entries []loki.Entry) {
				var r firehoseResponse
				require.NoError(t, json.Unmarshal(res.Body.Bytes(), &r))

				require.Equal(t, http.StatusOK, res.Code)
				require.Equal(t, testRequestID, r.RequestID)
				require.Equal(t, "hola esto es una prueba", entries[0].Line)
				require.Len(t, entries, 1)
			},
		},
		{
			Name:          "direct put data, using incoming timestamp",
			Body:          readTestData(t, "testdata/direct_put.json"),
			UseIncomingTs: true,
			Assert: func(t *testing.T, res *httptest.ResponseRecorder, entries []loki.Entry) {
				var r firehoseResponse
				require.NoError(t, json.Unmarshal(res.Body.Bytes(), &r))

				require.Equal(t, http.StatusOK, res.Code)
				require.Equal(t, testRequestID, r.RequestID)
				require.Len(t, entries, 3)
				expectedTimestamp := time.Unix(directPutRequestTimestamp/1000, 0)
				for _, e := range entries {
					require.Equal(t, expectedTimestamp, e.Timestamp)
				}
			},
		},
		{
			Name: "cloudwatch logs-subscription data",
			Body: readTestData(t, "testdata/cw_logs_mixed.json"),
			Assert: func(t *testing.T, res *httptest.ResponseRecorder, entries []loki.Entry) {
				var r firehoseResponse
				require.NoError(t, json.Unmarshal(res.Body.Bytes(), &r))

				require.Equal(t, http.StatusOK, res.Code)
				require.Equal(t, testRequestID, r.RequestID)
				require.Len(t, entries, 14)
				assertCloudwatchDataContents(t, entries, append(cwLambdaLogMessages, cwLambdaControlMessage)...)
				for _, e := range entries {
					require.NotContains(t, e.Labels, "__tenant_id__")
				}
			},
		},
		{
			Name:          "cloudwatch logs-subscription data, using incoming timestamp",
			Body:          readTestData(t, "testdata/cw_logs_mixed.json"),
			UseIncomingTs: true,
			Assert: func(t *testing.T, res *httptest.ResponseRecorder, entries []loki.Entry) {
				var r firehoseResponse
				require.NoError(t, json.Unmarshal(res.Body.Bytes(), &r))

				require.Equal(t, http.StatusOK, res.Code)
				require.Equal(t, testRequestID, r.RequestID)
				require.Len(t, entries, 14)
				for i, e := range entries {
					require.Equal(t, time.UnixMilli(cwLogsTimestamps[i]), e.Timestamp)
				}
			},
		},
		{
			Name:     "cloudwatch logs-subscription data, with tenant ID",
			Body:     readTestData(t, "testdata/cw_logs_with_only_control_messages.json"),
			TenantID: "20",
			Assert: func(t *testing.T, res *httptest.ResponseRecorder, entries []loki.Entry) {
				var r firehoseResponse
				require.NoError(t, json.Unmarshal(res.Body.Bytes(), &r))

				require.Equal(t, http.StatusOK, res.Code)
				require.Len(t, entries, 1)
				require.Equal(t, "20", string(entries[0].Labels["__tenant_id__"]))
			},
		},
		{
			Name: "cloudwatch logs-subscription data, relabeling control message",
			Body: readTestData(t, "testdata/cw_logs_with_only_control_messages.json"),
			Relabels: []*relabel.Config{
				keepLabelRule("__aws_owner", "aws_owner"),
				keepLabelRule("__aws_cw_msg_type", "msg_type"),
			},
			Assert: func(t *testing.T, res *httptest.ResponseRecorder, entries []loki.Entry) {
				var r firehoseResponse
				require.NoError(t, json.Unmarshal(res.Body.Bytes(), &r))

				require.Equal(t, http.StatusOK, res.Code)
				require.Equal(t, testRequestID, r.RequestID)
				require.Len(t, entries, 1)
				assertCloudwatchDataContents(t, entries, cwLambdaControlMessage)
				require.Equal(t, "CloudwatchLogs", string(entries[0].Labels["aws_owner"]))
				require.Equal(t, "CONTROL_MESSAGE", string(entries[0].Labels["msg_type"]))
			},
		},
		{
			Name: "cloudwatch logs-subscription data, relabeling log messages",
			Body: readTestData(t, "testdata/cw_logs_with_only_data_messages.json"),
			Relabels: []*relabel.Config{
				keepLabelRule("__aws_owner", "aws_owner"),
				keepLabelRule("__aws_cw_log_group", "log_group"),
				keepLabelRule("__aws_cw_log_stream", "log_stream"),
				keepLabelRule("__aws_cw_matched_filters", "filters"),
				keepLabelRule("__aws_cw_msg_type", "msg_type"),
			},
			Assert: func(t *testing.T, res *httptest.ResponseRecorder, entries []loki.Entry) {
				var r firehoseResponse
				require.NoError(t, json.Unmarshal(res.Body.Bytes(), &r))

				require.Equal(t, http.StatusOK, res.Code)
				require.Equal(t, testRequestID, r.RequestID)
				require.Len(t, entries, 13)
				assertCloudwatchDataContents(t, entries, cwLambdaLogMessages...)
				require.Equal(t, "366620023056", string(entries[0].Labels["aws_owner"]))
				require.Equal(t, "DATA_MESSAGE", string(entries[0].Labels["msg_type"]))
				require.Equal(t, "/aws/lambda/logging-lambda", string(entries[0].Labels["log_group"]))
				require.Equal(t, "2023/05/18/[$LATEST]405d340d30f844c4ad376392489343f5", string(entries[0].Labels["log_stream"]))
				require.Equal(t, "test_lambdafunction_logfilter", string(entries[0].Labels["filters"]))
			},
		},
		{
			Name: "non json payload",
			Body: `{`,
			Assert: func(t *testing.T, res *httptest.ResponseRecorder, _ []loki.Entry) {
				require.Equal(t, http.StatusBadRequest, res.Code)
			},
		},
		{
			Name: "cloudwatch logs control message, and invalid gzipped data",
			Body: readTestData(t, "testdata/cw_logs_control_and_bad_records.json"),
			Assert: func(t *testing.T, res *httptest.ResponseRecorder, entries []loki.Entry) {
				var r firehoseResponse
				require.NoError(t, json.Unmarshal(res.Body.Bytes(), &r))

				require.Equal(t, http.StatusOK, res.Code)
				require.Equal(t, testRequestID, r.RequestID)
				require.Len(t, entries, 1)
				assertCloudwatchDataContents(t, entries, cwLambdaControlMessage)
			},
			AssertMetrics: func(t *testing.T, ms []*dto.MetricFamily) {
				found := false
				for _, m := range ms {
					if *m.Name == "loki_source_awsfirehose_record_errors" {
						found = true
						require.Len(t, m.Metric, 1)
						require.Equal(t, float64(1), *m.Metric[0].Counter.Value)
						require.Len(t, m.Metric[0].Label, 1)
						lb := m.Metric[0].Label[0]
						require.Equal(t, "reason", *lb.Name)
						require.Equal(t, "base64-decode", *lb.Value)
					}
				}
				require.True(t, found)
			},
		},
	}

	for _, tc := range tests {
		for _, gzipContentEncoding := range []bool{true, false} {
			suffix := ""
			if gzipContentEncoding {
				suffix = " - with gzip content encoding"
			}

			t.Run(tc.Name+suffix, func(t *testing.T) {
				registry := prometheus.NewRegistry()
				route := &firehoseRoute{metrics: newMetrics(registry)}

				bodyReader := buildBodyReader(t, tc.Body, gzipContentEncoding)
				req := httptest.NewRequest(http.MethodPost, "http://test", bodyReader)
				req.Header.Set("X-Amz-Firehose-Request-Id", testRequestID)
				req.Header.Set("X-Amz-Firehose-Source-Arn", testSourceARN)
				req.Header.Set("X-Amz-Firehose-Protocol-Version", "1.0")
				req.Header.Set("User-Agent", "Amazon Kinesis Data Firehose Agent/1.0")
				if tc.TenantID != "" {
					req.Header.Set("X-Scope-OrgID", tc.TenantID)
				}
				if gzipContentEncoding {
					req.Header.Set("Content-Encoding", "gzip")
				}

				entries, status, err := route.Logs(req, &source.LogsConfig{
					RelabelRules:         tc.Relabels,
					UseIncomingTimestamp: tc.UseIncomingTs,
				})

				recorder := httptest.NewRecorder()
				route.WriteResponse(recorder, req, status, err)

				tc.Assert(t, recorder, entries)

				if tc.AssertMetrics != nil {
					gatheredMetrics, err := registry.Gather()
					require.NoError(t, err)
					tc.AssertMetrics(t, gatheredMetrics)
				}
			})
		}
	}
}

func TestRouteAuth(t *testing.T) {
	type testcase struct {
		Name         string
		accessKey    string
		reqAccessKey string
		expectedCode int
	}

	tests := []testcase{
		{
			Name:         "auth disabled",
			expectedCode: http.StatusOK,
		},
		{
			Name:         "auth enabled, valid key",
			accessKey:    "fakekey",
			reqAccessKey: "fakekey",
			expectedCode: http.StatusOK,
		},
		{
			Name:         "auth enabled, invalid key",
			accessKey:    "fakekey",
			reqAccessKey: "badkey",
			expectedCode: http.StatusUnauthorized,
		},
		{
			Name:         "auth enabled, no key",
			accessKey:    "fakekey",
			expectedCode: http.StatusUnauthorized,
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			route := &firehoseRoute{
				metrics:   newMetrics(prometheus.NewRegistry()),
				accessKey: tc.accessKey,
			}

			req := httptest.NewRequest(http.MethodPost, "http://test", strings.NewReader(readTestData(t, "testdata/direct_put.json")))
			req.Header.Set("X-Amz-Firehose-Request-Id", testRequestID)
			req.Header.Set("X-Amz-Firehose-Source-Arn", testSourceARN)
			req.Header.Set("X-Amz-Firehose-Protocol-Version", "1.0")
			req.Header.Set("User-Agent", "Amazon Kinesis Data Firehose Agent/1.0")
			if tc.reqAccessKey != "" {
				req.Header.Set("X-Amz-Firehose-Access-Key", tc.reqAccessKey)
			}

			_, status, err := route.Logs(req, &source.LogsConfig{})
			recorder := httptest.NewRecorder()
			route.WriteResponse(recorder, req, status, err)

			require.Equal(t, tc.expectedCode, recorder.Code)
		})
	}
}

func TestRouteWithStaticConfigLabels(t *testing.T) {
	type testcase struct {
		Name               string
		tenantID           string
		body               string
		staticLabelsConfig string
		assert             func(t *testing.T, res *httptest.ResponseRecorder, entries []loki.Entry)
	}

	tests := []testcase{
		{
			Name: "direct put data, static labels",
			body: readTestData(t, "testdata/direct_put.json"),
			staticLabelsConfig: `
				{
				  "commonAttributes": {
					"lbl_mylabel1": "myvalue1",
					"lbl_mylabel2": "myvalue2"
				  }
				}
			`,
			assert: func(t *testing.T, res *httptest.ResponseRecorder, entries []loki.Entry) {
				var r firehoseResponse
				require.NoError(t, json.Unmarshal(res.Body.Bytes(), &r))
				require.Equal(t, http.StatusOK, res.Code)
				require.Len(t, entries, 3)
				for _, e := range entries {
					require.Equal(t, "myvalue1", string(e.Labels["mylabel1"]))
					require.Equal(t, "myvalue2", string(e.Labels["mylabel2"]))
				}
			},
		},
		{
			Name: "cloudwatch logs-subscription data, static labels",
			body: readTestData(t, "testdata/cw_logs_with_only_control_messages.json"),
			staticLabelsConfig: `
				{
				  "commonAttributes": {
					"lbl_mylabel1": "myvalue1",
					"lbl_mylabel2": "myvalue2"
				  }
				}
			`,
			assert: func(t *testing.T, res *httptest.ResponseRecorder, entries []loki.Entry) {
				var r firehoseResponse
				require.NoError(t, json.Unmarshal(res.Body.Bytes(), &r))
				require.Len(t, entries, 1)
				assertCloudwatchDataContents(t, entries, cwLambdaControlMessage)
				require.Equal(t, "myvalue1", string(entries[0].Labels["mylabel1"]))
				require.Equal(t, "myvalue2", string(entries[0].Labels["mylabel2"]))
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			route := &firehoseRoute{metrics: newMetrics(prometheus.NewRegistry())}
			req := httptest.NewRequest(http.MethodPost, "https://example.com", strings.NewReader(tc.body))
			req.Header.Set("X-Amz-Firehose-Request-Id", testRequestID)
			req.Header.Set("X-Amz-Firehose-Source-Arn", testSourceARN)
			req.Header.Set("X-Amz-Firehose-Protocol-Version", "1.0")
			req.Header.Set(commonAttributesHeader, tc.staticLabelsConfig)
			req.Header.Set("User-Agent", "Amazon Kinesis Data Firehose Agent/1.0")
			if tc.tenantID != "" {
				req.Header.Set("X-Scope-OrgID", tc.tenantID)
			}

			entries, status, err := route.Logs(req, &source.LogsConfig{})
			recorder := httptest.NewRecorder()
			route.WriteResponse(recorder, req, status, err)
			tc.assert(t, recorder, entries)
		})
	}
}

func TestGetStaticLabelsFromRequest(t *testing.T) {
	type testcase struct {
		name   string
		config string
		want   model.LabelSet
	}

	tests := []testcase{
		{
			name: "single label",
			config: `
				{
				  "commonAttributes": {
					"lbl_label1": "value1"
				  }
				}
			`,
			want: model.LabelSet{"label1": "value1"},
		},
		{
			name: "multiple labels",
			config: `
				{
				  "commonAttributes": {
					"lbl_label1": "value1",
					"lbl_label2": "value2"
				  }
				}
			`,
			want: model.LabelSet{"label1": "value1", "label2": "value2"},
		},
		{
			name:   "empty config",
			config: ``,
			want:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			route := &firehoseRoute{}
			req := httptest.NewRequest(http.MethodGet, "https://example.com", nil)
			req.Header.Set(commonAttributesHeader, tt.config)
			req.Header.Set("X-Scope-OrgID", "001")
			got := route.tryToGetStaticLabelsFromRequest(req, "001")
			require.Equal(t, tt.want, got)
		})
	}
}

func TestGetStaticLabelsFromRequest_NoError_InvalidData(t *testing.T) {
	type testcase struct {
		name            string
		config          string
		want            model.LabelSet
		expectedMetrics string
	}

	tests := []testcase{
		{
			name:   "invalid config",
			config: `!@#$%^&*()_`,
			expectedMetrics: `
				# HELP loki_source_awsfirehose_invalid_static_labels_errors Number of errors while processing AWS Firehose static labels
				# TYPE loki_source_awsfirehose_invalid_static_labels_errors counter
				loki_source_awsfirehose_invalid_static_labels_errors{reason="invalid_json_format",tenant_id="001"} 1
			`,
			want: model.LabelSet(nil),
		},
		{
			name: "invalid label name",
			config: `
				{
				  "commonAttributes": {
					"lbl_l@bel1": "value1"
				  }
				}
			`,
			want: model.LabelSet{"l_bel1": "value1"},
		},
		{
			name: "invalid label name, mixed case",
			config: `
				{
				  "commonAttributes": {
					"lbl_L@bEl1%": "value1"
				  }
				}
			`,
			want: model.LabelSet{"l_b_el1_percent": "value1"},
		},
		{
			name: "invalid label name",
			config: `
				{
				  "commonAttributes": {
					"\xed\xa0\x80\x80": "value1"
				  }
				}
			`,
			expectedMetrics: `
				# HELP loki_source_awsfirehose_invalid_static_labels_errors Number of errors while processing AWS Firehose static labels
				# TYPE loki_source_awsfirehose_invalid_static_labels_errors counter
				loki_source_awsfirehose_invalid_static_labels_errors{reason="invalid_json_format",tenant_id="001"} 1
			`,
			want: model.LabelSet(nil),
		},
		{
			name: "invalid label value, invalid JSON",
			config: `
				{
				  "commonAttributes": {
					"label1": "\xed\xa0\x80\x80"
				  }
				}
			`,
			expectedMetrics: `
				# HELP loki_source_awsfirehose_invalid_static_labels_errors Number of errors while processing AWS Firehose static labels
				# TYPE loki_source_awsfirehose_invalid_static_labels_errors counter
				loki_source_awsfirehose_invalid_static_labels_errors{reason="invalid_json_format",tenant_id="001"} 1
			`,
			want: model.LabelSet(nil),
		},
		{
			name: "invalid label",
			config: `
				{
				  "commonAttributes": {
					"lbl_0mylable": "value"
				  }
				}
			`,
			expectedMetrics: `
				# HELP loki_source_awsfirehose_invalid_static_labels_errors Number of errors while processing AWS Firehose static labels
				# TYPE loki_source_awsfirehose_invalid_static_labels_errors counter
				loki_source_awsfirehose_invalid_static_labels_errors{reason="invalid_label_name",tenant_id="001"} 1
			`,
			want: model.LabelSet{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry := prometheus.NewRegistry()
			route := &firehoseRoute{metrics: newMetrics(registry)}
			req := httptest.NewRequest(http.MethodGet, "https://example.com", nil)
			req.Header.Set(commonAttributesHeader, tt.config)
			req.Header.Set("X-Scope-OrgID", "001")
			got := route.tryToGetStaticLabelsFromRequest(req, "001")

			require.Equal(t, tt.want, got)
			if tt.expectedMetrics != "" {
				err := testutil.GatherAndCompare(registry, strings.NewReader(tt.expectedMetrics), "loki_source_awsfirehose_invalid_static_labels_errors")
				require.NoError(t, err)
			}
		})
	}
}

const cwLambdaControlMessage = `CWL CONTROL MESSAGE: Checking health of destination Firehose.`

var cwLambdaLogMessages = []string{
	"INIT_START Runtime Version: nodejs:18.v6\tRuntime Version ARN: arn:aws:lambda:us-east-2::runtime:813a1c9d8f27c16e2f3288da6255eac7867411c306ae9cf76498bb320eddded2\n",
	"START RequestId: 632d3270-354e-4504-96e1-e3a74218c002 Version: $LATEST\n",
	"2023-05-18T15:33:23.822Z\t632d3270-354e-4504-96e1-e3a74218c002\tINFO\thello i'm a lambda and its 1684424003821\n",
	"END RequestId: 632d3270-354e-4504-96e1-e3a74218c002\n",
	"REPORT RequestId: 632d3270-354e-4504-96e1-e3a74218c002\tDuration: 37.18 ms\tBilled Duration: 38 ms\tMemory Size: 128 MB\tMax Memory Used: 65 MB\tInit Duration: 177.89 ms\t\n",
	"START RequestId: 261fbfb2-8a5f-4977-b6a6-e701a622ee16 Version: $LATEST\n",
	"2023-05-18T15:33:25.708Z\t261fbfb2-8a5f-4977-b6a6-e701a622ee16\tINFO\thello i'm a lambda and its 1684424005707\n",
	"END RequestId: 261fbfb2-8a5f-4977-b6a6-e701a622ee16\n",
	"REPORT RequestId: 261fbfb2-8a5f-4977-b6a6-e701a622ee16\tDuration: 11.61 ms\tBilled Duration: 12 ms\tMemory Size: 128 MB\tMax Memory Used: 66 MB\t\n",
	"START RequestId: 921a2a6d-5bd1-4797-8400-4688494b664b Version: $LATEST\n",
	"2023-05-18T15:33:27.493Z\t921a2a6d-5bd1-4797-8400-4688494b664b\tINFO\thello i'm a lambda and its 1684424007493\n",
	"END RequestId: 921a2a6d-5bd1-4797-8400-4688494b664b\n",
	"REPORT RequestId: 921a2a6d-5bd1-4797-8400-4688494b664b\tDuration: 1.74 ms\tBilled Duration: 2 ms\tMemory Size: 128 MB\tMax Memory Used: 66 MB\t\n",
}

func assertCloudwatchDataContents(t *testing.T, entries []loki.Entry, expectedLines ...string) {
	t.Helper()

	seen := make(map[string]bool, len(expectedLines))
	for _, l := range expectedLines {
		seen[l] = false
	}
	for _, entry := range entries {
		seen[entry.Line] = true
	}
	for line, wasSeen := range seen {
		require.True(t, wasSeen, "line '%s' was not seen", line)
	}
}

func keepLabelRule(src, dst string) *relabel.Config {
	return &relabel.Config{
		SourceLabels:         model.LabelNames{model.LabelName(src)},
		Regex:                relabel.MustNewRegexp("(.*)"),
		Replacement:          "$1",
		TargetLabel:          dst,
		Action:               relabel.Replace,
		NameValidationScheme: model.LegacyValidation,
	}
}

func buildBodyReader(t *testing.T, body string, gzipContentEncoding bool) io.Reader {
	t.Helper()

	if !gzipContentEncoding {
		return strings.NewReader(body)
	}

	bs := bytes.NewBuffer(nil)
	gzipWriter := gzip.NewWriter(bs)
	_, err := io.Copy(gzipWriter, strings.NewReader(body))
	require.NoError(t, err)
	require.NoError(t, gzipWriter.Close())
	return bs
}
