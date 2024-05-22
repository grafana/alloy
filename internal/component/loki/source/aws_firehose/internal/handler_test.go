package internal

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/go-kit/log"
	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/klauspost/compress/gzip"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/relabel"
	"github.com/stretchr/testify/require"
)

const (
	testRequestID = "86208cf6-2bcc-47e6-9010-02ca9f44a025"
	testSourceARN = "arn:aws:firehose:us-east-2:123:deliverystream/aws_firehose_test_stream"

	directPutRequestTimestamp = 1684422829730
	cwRequestTimestamp        = 1684424042901
)

//go:embed testdata/*
var testData embed.FS

func readTestData(t *testing.T, name string) string {
	f, err := testData.ReadFile(name)
	if err != nil {
		require.FailNow(t, fmt.Sprintf("error reading test data: %s", name))
	}
	return string(f)
}

type receiver struct {
	entries []loki.Entry
}

func (r *receiver) Send(ctx context.Context, entry loki.Entry) {
	r.entries = append(r.entries, entry)
}

type response struct {
	RequestID string `json:"requestId"`
}

func TestHandler(t *testing.T) {
	type testcase struct {
		// TenantID configures the X-Scope-OrgID header in the test request when present.
		TenantID string

		// UseIncomingTs configures the handler under test to use or not the incoming request timestamp
		UseIncomingTs bool

		// Body is the payload of the request.
		Body string

		// Relabels are the relabeling rules configured on Handler.
		Relabels []*relabel.Config

		// Assert is the main assertion function ran after the request is successful.
		Assert func(t *testing.T, res *httptest.ResponseRecorder, entries []loki.Entry)

		// AssertMetrics is an optional assertion over the collected metrics
		AssertMetrics func(t *testing.T, m []*dto.MetricFamily)
	}

	tests := map[string]testcase{
		"direct put data": {
			Body: readTestData(t, "testdata/direct_put.json"),
			Assert: func(t *testing.T, res *httptest.ResponseRecorder, entries []loki.Entry) {
				r := response{}
				require.NoError(t, json.Unmarshal(res.Body.Bytes(), &r))

				require.Equal(t, 200, res.Code)
				require.Equal(t, "a1af4300-6c09-4916-ba8f-12f336176246", r.RequestID)
				require.Len(t, entries, 3)
				for _, e := range entries {
					// only add special tenant label if present
					require.NotContains(t, e.Labels, "__tenant_id__")
				}
			},
		},
		"direct put data, with tenant ID": {
			Body:     readTestData(t, "testdata/direct_put.json"),
			TenantID: "20",
			Assert: func(t *testing.T, res *httptest.ResponseRecorder, entries []loki.Entry) {
				r := response{}
				require.NoError(t, json.Unmarshal(res.Body.Bytes(), &r))

				require.Equal(t, 200, res.Code)
				require.Len(t, entries, 3)
				for _, e := range entries {
					require.Equal(t, "20", string(e.Labels["__tenant_id__"]))
				}
			},
		},
		"direct put data, relabeling req id and source arn": {
			Body: readTestData(t, "testdata/direct_put.json"),
			Relabels: []*relabel.Config{
				{
					SourceLabels: model.LabelNames{"__aws_firehose_request_id"},
					Regex:        relabel.MustNewRegexp("(.*)"),
					Replacement:  "$1",
					TargetLabel:  "aws_request_id",
					Action:       relabel.Replace,
				},
				{
					SourceLabels: model.LabelNames{"__aws_firehose_source_arn"},
					Regex:        relabel.MustNewRegexp("(.*)"),
					Replacement:  "$1",
					TargetLabel:  "aws_source_arn",
					Action:       relabel.Replace,
				},
			},
			Assert: func(t *testing.T, res *httptest.ResponseRecorder, entries []loki.Entry) {
				r := response{}
				require.NoError(t, json.Unmarshal(res.Body.Bytes(), &r))

				require.Equal(t, 200, res.Code)
				require.Equal(t, "a1af4300-6c09-4916-ba8f-12f336176246", r.RequestID)
				require.Len(t, entries, 3)

				for _, e := range entries {
					require.Equal(t, testRequestID, string(e.Labels["aws_request_id"]))
					require.Equal(t, testSourceARN, string(e.Labels["aws_source_arn"]))
				}
			},
		},
		"direct put data with non JSON data": {
			Body: readTestData(t, "testdata/direct_put_with_non_json_message.json"),
			Assert: func(t *testing.T, res *httptest.ResponseRecorder, entries []loki.Entry) {
				r := response{}
				require.NoError(t, json.Unmarshal(res.Body.Bytes(), &r))

				require.Equal(t, 200, res.Code)
				require.Equal(t, "aa9febd3-d9d0-45a2-9032-294078d926d5", r.RequestID)
				require.Equal(t, "hola esto es una prueba", entries[0].Line)
				require.Len(t, entries, 1)
			},
		},
		"direct put data, using incoming timestamp": {
			Body:          readTestData(t, "testdata/direct_put.json"),
			UseIncomingTs: true,
			Assert: func(t *testing.T, res *httptest.ResponseRecorder, entries []loki.Entry) {
				r := response{}
				require.NoError(t, json.Unmarshal(res.Body.Bytes(), &r))

				require.Equal(t, 200, res.Code)
				require.Equal(t, "a1af4300-6c09-4916-ba8f-12f336176246", r.RequestID)
				require.Len(t, entries, 3)
				expectedTimestamp := time.Unix(directPutRequestTimestamp/1000, 0)
				for _, e := range entries {
					require.Equal(t, expectedTimestamp, e.Timestamp, "timestamp is other than expected")
				}
			},
		},
		"cloudwatch logs-subscription data": {
			Body: readTestData(t, "testdata/cw_logs_mixed.json"),
			Assert: func(t *testing.T, res *httptest.ResponseRecorder, entries []loki.Entry) {
				r := response{}
				require.NoError(t, json.Unmarshal(res.Body.Bytes(), &r))

				require.Equal(t, 200, res.Code)
				require.Equal(t, "86208cf6-2bcc-47e6-9010-02ca9f44a025", r.RequestID)

				require.Len(t, entries, 14)
				// assert that all expected lines were seen
				assertCloudwatchDataContents(t, res, entries, append(cwLambdaLogMessages, cwLambdaControlMessage)...)
				for _, e := range entries {
					// only add special tenant label if present
					require.NotContains(t, e.Labels, "__tenant_id__")
				}
			},
		},
		"cloudwatch logs-subscription data, using incoming timestamp": {
			Body:          readTestData(t, "testdata/cw_logs_mixed.json"),
			UseIncomingTs: true,
			Assert: func(t *testing.T, res *httptest.ResponseRecorder, entries []loki.Entry) {
				r := response{}
				require.NoError(t, json.Unmarshal(res.Body.Bytes(), &r))

				require.Equal(t, 200, res.Code)
				require.Equal(t, "86208cf6-2bcc-47e6-9010-02ca9f44a025", r.RequestID)

				require.Len(t, entries, 14)
				expectedTimestamp := time.Unix(cwRequestTimestamp/1000, 0)
				for _, e := range entries {
					require.Equal(t, expectedTimestamp, e.Timestamp, "timestamp is other than expected")
				}
			},
		},
		"cloudwatch logs-subscription data, with tenant ID": {
			Body:     readTestData(t, "testdata/cw_logs_with_only_control_messages.json"),
			TenantID: "20",
			Assert: func(t *testing.T, res *httptest.ResponseRecorder, entries []loki.Entry) {
				r := response{}
				require.NoError(t, json.Unmarshal(res.Body.Bytes(), &r))

				require.Equal(t, 200, res.Code)

				require.Len(t, entries, 1)
				require.Equal(t, "20", string(entries[0].Labels["__tenant_id__"]))
			},
		},
		"cloudwatch logs-subscription data, relabeling control message": {
			Body: readTestData(t, "testdata/cw_logs_with_only_control_messages.json"),
			Relabels: []*relabel.Config{
				keepLabelRule("__aws_owner", "aws_owner"),
				keepLabelRule("__aws_cw_msg_type", "msg_type"),
			},
			Assert: func(t *testing.T, res *httptest.ResponseRecorder, entries []loki.Entry) {
				r := response{}
				require.NoError(t, json.Unmarshal(res.Body.Bytes(), &r))

				require.Equal(t, 200, res.Code)
				require.Equal(t, "86208cf6-2bcc-47e6-9010-02ca9f44a025", r.RequestID)

				require.Len(t, entries, 1)
				// assert that all expected lines were seen
				assertCloudwatchDataContents(t, res, entries, cwLambdaControlMessage)

				require.Equal(t, "CloudwatchLogs", string(entries[0].Labels["aws_owner"]))
				require.Equal(t, "CONTROL_MESSAGE", string(entries[0].Labels["msg_type"]))
			},
		},
		"cloudwatch logs-subscription data, relabeling log messages": {
			Body: readTestData(t, "testdata/cw_logs_with_only_data_messages.json"),
			Relabels: []*relabel.Config{
				keepLabelRule("__aws_owner", "aws_owner"),
				keepLabelRule("__aws_cw_log_group", "log_group"),
				keepLabelRule("__aws_cw_log_stream", "log_stream"),
				keepLabelRule("__aws_cw_matched_filters", "filters"),
				keepLabelRule("__aws_cw_msg_type", "msg_type"),
			},
			Assert: func(t *testing.T, res *httptest.ResponseRecorder, entries []loki.Entry) {
				r := response{}
				require.NoError(t, json.Unmarshal(res.Body.Bytes(), &r))

				require.Equal(t, 200, res.Code)
				require.Equal(t, "86208cf6-2bcc-47e6-9010-02ca9f44a025", r.RequestID)

				require.Len(t, entries, 13)
				// assert that all expected lines were seen
				assertCloudwatchDataContents(t, res, entries, cwLambdaLogMessages...)

				require.Equal(t, "366620023056", string(entries[0].Labels["aws_owner"]))
				require.Equal(t, "DATA_MESSAGE", string(entries[0].Labels["msg_type"]))
				require.Equal(t, "/aws/lambda/logging-lambda", string(entries[0].Labels["log_group"]))
				require.Equal(t, "/aws/lambda/logging-lambda", string(entries[0].Labels["log_group"]))
				require.Equal(t, "2023/05/18/[$LATEST]405d340d30f844c4ad376392489343f5", string(entries[0].Labels["log_stream"]))
				require.Equal(t, "test_lambdafunction_logfilter", string(entries[0].Labels["filters"]))
			},
		},
		"non json payload": {
			Body: `{`,
			Assert: func(t *testing.T, res *httptest.ResponseRecorder, entries []loki.Entry) {
				require.Equal(t, 400, res.Code)
			},
		},
		"cloudwatch logs control message, and invalid gzipped data": {
			Body: readTestData(t, "testdata/cw_logs_control_and_bad_records.json"),
			Assert: func(t *testing.T, res *httptest.ResponseRecorder, entries []loki.Entry) {
				r := response{}
				require.NoError(t, json.Unmarshal(res.Body.Bytes(), &r))

				require.Equal(t, 200, res.Code)
				require.Equal(t, "86208cf6-2bcc-47e6-9010-02ca9f44a025", r.RequestID)

				require.Len(t, entries, 1)
				// assert that all expected lines were seen
				assertCloudwatchDataContents(t, res, entries, cwLambdaControlMessage)
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

	for name, tc := range tests {
		for _, gzipContentEncoding := range []bool{true, false} {
			suffix := ""
			if gzipContentEncoding {
				suffix = " - with gzip content encoding"
			}
			t.Run(fmt.Sprintf("%s%s", name, suffix), func(t *testing.T) {
				w := log.NewSyncWriter(os.Stderr)
				logger := log.NewLogfmtLogger(w)

				testReceiver := &receiver{entries: make([]loki.Entry, 0)}
				registry := prometheus.NewRegistry()
				accessKey := ""
				handler := NewHandler(testReceiver, logger, NewMetrics(registry), tc.Relabels, tc.UseIncomingTs, accessKey)

				bs := bytes.NewBuffer(nil)
				var bodyReader io.Reader = strings.NewReader(tc.Body)

				// if testing gzip content encoding, use the following read/writer chain
				// to compress the body: string reader -> gzip writer -> bytes buffer
				// after that use the same bytes buffer as reader
				if gzipContentEncoding {
					gzipWriter := gzip.NewWriter(bs)
					_, err := io.Copy(gzipWriter, bodyReader)
					require.NoError(t, err)
					require.NoError(t, gzipWriter.Close())
					bodyReader = bs
				}

				req, err := http.NewRequest("POST", "http://test", bodyReader)
				req.Header.Set("X-Amz-Firehose-Request-Id", testRequestID)
				req.Header.Set("X-Amz-Firehose-Source-Arn", testSourceARN)
				req.Header.Set("X-Amz-Firehose-Protocol-Version", "1.0")
				req.Header.Set("User-Agent", "Amazon Kinesis Data Firehose Agent/1.0")
				if tc.TenantID != "" {
					req.Header.Set("X-Scope-OrgID", tc.TenantID)
				}
				require.NoError(t, err)

				// Also content-encoding header needs to be set
				if gzipContentEncoding {
					req.Header.Set("Content-Encoding", "gzip")
				}

				recorder := httptest.NewRecorder()
				handler.ServeHTTP(recorder, req)

				// delegate assertions
				tc.Assert(t, recorder, testReceiver.entries)

				if tc.AssertMetrics != nil {
					gatheredMetrics, err := registry.Gather()
					require.NoError(t, err)
					tc.AssertMetrics(t, gatheredMetrics)
				}
			})
		}
	}
}

func TestHandlerAuth(t *testing.T) {
	type testcase struct {
		// AccessKey configures the key required by the handler to accept requests
		AccessKey string

		// ReqAccessKey configures the key sent in the request
		ReqAccessKey string

		// ExpectedCode is the expected HTTP status code
		ExpectedCode int
	}

	tests := map[string]testcase{
		"auth disabled": {
			AccessKey:    "",
			ReqAccessKey: "",
			ExpectedCode: 200,
		},
		"auth enabled, valid key": {
			AccessKey:    "fakekey",
			ReqAccessKey: "fakekey",
			ExpectedCode: 200,
		},
		"auth enabled, invalid key": {
			AccessKey:    "fakekey",
			ReqAccessKey: "badkey",
			ExpectedCode: 401,
		},
		"auth enabled, no key": {
			AccessKey:    "fakekey",
			ReqAccessKey: "",
			ExpectedCode: 401,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			w := log.NewSyncWriter(os.Stderr)
			logger := log.NewLogfmtLogger(w)

			testReceiver := &receiver{entries: make([]loki.Entry, 0)}
			registry := prometheus.NewRegistry()
			relabeling := []*relabel.Config{}
			incommingTs := false
			handler := NewHandler(testReceiver, logger, NewMetrics(registry), relabeling, incommingTs, tc.AccessKey)

			body := strings.NewReader(readTestData(t, "testdata/direct_put.json"))
			req, err := http.NewRequest("POST", "http://test", body)
			req.Header.Set("X-Amz-Firehose-Request-Id", testRequestID)
			req.Header.Set("X-Amz-Firehose-Source-Arn", testSourceARN)
			req.Header.Set("X-Amz-Firehose-Protocol-Version", "1.0")
			req.Header.Set("User-Agent", "Amazon Kinesis Data Firehose Agent/1.0")
			if tc.ReqAccessKey != "" {
				req.Header.Set("X-Amz-Firehose-Access-Key", tc.ReqAccessKey)
			}
			require.NoError(t, err)

			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, req)

			require.Equal(t, tc.ExpectedCode, recorder.Code)
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

func assertCloudwatchDataContents(t *testing.T, _ *httptest.ResponseRecorder, entries []loki.Entry, expectedLines ...string) {
	var seen = make(map[string]bool)
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
		SourceLabels: model.LabelNames{model.LabelName(src)},
		Regex:        relabel.MustNewRegexp("(.*)"),
		Replacement:  "$1",
		TargetLabel:  dst,
		Action:       relabel.Replace,
	}
}

func TestHandlerWithStaticConfigsLabels(t *testing.T) {
	type testcase struct {
		// TenantID configures the X-Scope-OrgID header in the test request when present.
		TenantID string

		// Body is the payload of the request.
		Body string

		// Assert is the main assertion function ran after the request is successful.
		Assert func(t *testing.T, res *httptest.ResponseRecorder, entries []loki.Entry)

		// AssertMetrics is an optional assertion over the collected metrics
		AssertMetrics      func(t *testing.T, m []*dto.MetricFamily)
		StaticLabelsConfig string
	}

	tests := map[string]testcase{
		"direct put data, static labels": {
			Body: readTestData(t, "testdata/direct_put.json"),
			StaticLabelsConfig: `
				{
				  "commonAttributes": {
					"lbl_mylabel1": "myvalue1",
					"lbl_mylabel2": "myvalue2"
				  }
				}
			`,
			Assert: func(t *testing.T, res *httptest.ResponseRecorder, entries []loki.Entry) {
				r := response{}
				require.NoError(t, json.Unmarshal(res.Body.Bytes(), &r))

				require.Equal(t, 200, res.Code)
				require.Len(t, entries, 3)

				for _, e := range entries {
					require.Equal(t, "myvalue1", string(e.Labels["mylabel1"]))
					require.Equal(t, "myvalue2", string(e.Labels["mylabel2"]))
				}
			},
		},
		"cloudwatch logs-subscription data, static labels": {
			Body: readTestData(t, "testdata/cw_logs_with_only_control_messages.json"),
			StaticLabelsConfig: `
				{
				  "commonAttributes": {
					"lbl_mylabel1": "myvalue1",
					"lbl_mylabel2": "myvalue2"
				  }
				}
			`,
			Assert: func(t *testing.T, res *httptest.ResponseRecorder, entries []loki.Entry) {
				r := response{}
				require.NoError(t, json.Unmarshal(res.Body.Bytes(), &r))

				require.Len(t, entries, 1)
				// assert that all expected lines were seen
				assertCloudwatchDataContents(t, res, entries, cwLambdaControlMessage)

				require.Equal(t, "myvalue1", string(entries[0].Labels["mylabel1"]))
				require.Equal(t, "myvalue2", string(entries[0].Labels["mylabel2"]))
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			w := log.NewSyncWriter(os.Stderr)
			logger := log.NewLogfmtLogger(w)

			testReceiver := &receiver{entries: make([]loki.Entry, 0)}
			registry := prometheus.NewRegistry()
			accessKey := ""
			handler := NewHandler(testReceiver, logger, NewMetrics(registry), nil, false, accessKey)

			var bodyReader io.Reader = strings.NewReader(tc.Body)

			req, err := http.NewRequest("POST", "https://example.com", bodyReader)
			req.Header.Set("X-Amz-Firehose-Request-Id", testRequestID)
			req.Header.Set("X-Amz-Firehose-Source-Arn", testSourceARN)
			req.Header.Set("X-Amz-Firehose-Protocol-Version", "1.0")
			req.Header.Set(commonAttributesHeader, tc.StaticLabelsConfig)
			req.Header.Set("User-Agent", "Amazon Kinesis Data Firehose Agent/1.0")
			if tc.TenantID != "" {
				req.Header.Set("X-Scope-OrgID", tc.TenantID)
			}
			require.NoError(t, err)

			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, req)

			// delegate assertions
			tc.Assert(t, recorder, testReceiver.entries)

			if tc.AssertMetrics != nil {
				gatheredMetrics, err := registry.Gather()
				require.NoError(t, err)
				tc.AssertMetrics(t, gatheredMetrics)
			}
		})
	}
}

func TestGetStaticLabelsFromRequest(t *testing.T) {
	tests := []struct {
		name   string
		config string
		want   model.LabelSet
	}{
		{
			name: "single label",
			config: `
				{
				  "commonAttributes": {
					"lbl_label1": "value1"
				  }
				}
			`,
			want: model.LabelSet{
				"label1": "value1",
			},
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
			want: model.LabelSet{
				"label1": "value1",
				"label2": "value2",
			},
		},
		{
			name:   "empty config",
			config: ``,
			want:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := &Handler{}

			req := httptest.NewRequest(http.MethodGet, "https://example.com", nil)
			req.Header.Set(commonAttributesHeader, tt.config)
			req.Header.Set("X-Scope-OrgID", "001")
			got := handler.tryToGetStaticLabelsFromRequest(req, "001")

			require.Equal(t, tt.want, got)
		})
	}
}

func TestGetStaticLabelsFromRequest_NoError_InvalidData(t *testing.T) {
	tests := []struct {
		name            string
		config          string
		want            model.LabelSet
		expectedMetrics string
	}{
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

			want: model.LabelSet{
				"l_bel1": "value1",
			},
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
			want: model.LabelSet{
				"l_b_el1_percent": "value1",
			},
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
			w := log.NewSyncWriter(os.Stderr)
			logger := log.NewLogfmtLogger(w)

			testReceiver := &receiver{entries: make([]loki.Entry, 0)}
			registry := prometheus.NewRegistry()
			accessKey := ""
			handler := NewHandler(testReceiver, logger, NewMetrics(registry), nil, false, accessKey)

			req := httptest.NewRequest(http.MethodGet, "https://example.com", nil)
			req.Header.Set(commonAttributesHeader, tt.config)
			req.Header.Set("X-Scope-OrgID", "001")
			got := handler.tryToGetStaticLabelsFromRequest(req, "001")

			require.Equal(t, tt.want, got)

			err := testutil.GatherAndCompare(registry, strings.NewReader(tt.expectedMetrics), "loki_source_awsfirehose_invalid_static_labels_errors")
			require.NoError(t, err)
		})
	}
}
