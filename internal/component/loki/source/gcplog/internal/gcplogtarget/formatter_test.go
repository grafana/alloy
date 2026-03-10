package gcplogtarget

import (
	"testing"
	"time"

	"cloud.google.com/go/pubsub/v2"
	"github.com/grafana/loki/pkg/push"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/relabel"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component/common/loki"
)

const (
	withAllFields   = `{"logName": "https://project/gcs", "severity": "INFO", "resource": {"type": "gcs", "labels": {"backendServiceName": "http-loki", "bucketName": "loki-bucket", "instanceId": "344555"}}, "timestamp": "2020-12-22T15:01:23.045123456Z", "labels": {"dataflow.googleapis.com/region": "europe-west1"}}`
	logTextPayload  = "text-payload-log"
	withTextPayload = `{"logName": "https://project/gcs", "severity": "INFO", "textPayload": "` + logTextPayload + `", "resource": {"type": "gcs", "labels": {"backendServiceName": "http-loki", "bucketName": "loki-bucket", "instanceId": "344555"}}, "timestamp": "2020-12-22T15:01:23.045123456Z", "labels": {"dataflow.googleapis.com/region": "europe-west1"}}`
)

func TestParseLogEntry(t *testing.T) {
	type testCase struct {
		name     string
		msg      *pubsub.Message
		relabel  []*relabel.Config
		opts     parseOptions
		expected loki.Entry
	}

	tests := []testCase{
		{
			name: "relabelling",
			msg: &pubsub.Message{
				Data: []byte(withAllFields),
			},
			opts: parseOptions{
				useIncomingTimestamp: true,
				fixedLabels: model.LabelSet{
					"jobname": "pubsub-test",
				},
			},
			relabel: []*relabel.Config{
				{
					SourceLabels:         model.LabelNames{"__gcp_resource_labels_backend_service_name"},
					Separator:            ";",
					Regex:                relabel.MustNewRegexp("(.*)"),
					TargetLabel:          "backend_service_name",
					Action:               "replace",
					Replacement:          "$1",
					NameValidationScheme: model.LegacyValidation,
				},
				{
					SourceLabels:         model.LabelNames{"__gcp_resource_labels_bucket_name"},
					Separator:            ";",
					Regex:                relabel.MustNewRegexp("(.*)"),
					TargetLabel:          "bucket_name",
					Action:               "replace",
					Replacement:          "$1",
					NameValidationScheme: model.LegacyValidation,
				},
				{
					SourceLabels:         model.LabelNames{"__gcp_severity"},
					Separator:            ";",
					Regex:                relabel.MustNewRegexp("(.*)"),
					TargetLabel:          "severity",
					Action:               "replace",
					Replacement:          "$1",
					NameValidationScheme: model.LegacyValidation,
				},
				{
					SourceLabels:         model.LabelNames{"__gcp_labels_dataflow_googleapis_com_region"},
					Separator:            ";",
					Regex:                relabel.MustNewRegexp("(.*)"),
					TargetLabel:          "region",
					Action:               "replace",
					Replacement:          "$1",
					NameValidationScheme: model.LegacyValidation,
				},
			},
			expected: loki.Entry{
				Labels: model.LabelSet{
					"jobname":              "pubsub-test",
					"backend_service_name": "http-loki",
					"bucket_name":          "loki-bucket",
					"severity":             "INFO",
					"region":               "europe-west1",
				},
				Entry: push.Entry{
					Timestamp: mustTime(t, "2020-12-22T15:01:23.045123456Z"),
					Line:      withAllFields,
				},
			},
		},
		{
			name: "use-original-timestamp",
			msg: &pubsub.Message{
				Data: []byte(withAllFields),
			},
			opts: parseOptions{
				useIncomingTimestamp: true,
				fixedLabels: model.LabelSet{
					"jobname": "pubsub-test",
				},
			},
			expected: loki.Entry{
				Labels: model.LabelSet{
					"jobname": "pubsub-test",
				},
				Entry: push.Entry{
					Timestamp: mustTime(t, "2020-12-22T15:01:23.045123456Z"),
					Line:      withAllFields,
				},
			},
		},
		{
			name: "rewrite-timestamp",
			msg: &pubsub.Message{
				Data: []byte(withAllFields),
			},
			opts: parseOptions{
				fixedLabels: model.LabelSet{
					"jobname": "pubsub-test",
				},
			},
			expected: loki.Entry{
				Labels: model.LabelSet{
					"jobname": "pubsub-test",
				},
				Entry: push.Entry{
					Timestamp: time.Now(),
					Line:      withAllFields,
				},
			},
		},
		{
			name: "use-full-line",
			opts: parseOptions{
				useFullLine: true,
				fixedLabels: model.LabelSet{
					"jobname": "pubsub-test",
				},
			},
			msg: &pubsub.Message{
				Data: []byte(withTextPayload),
			},
			expected: loki.Entry{
				Labels: model.LabelSet{
					"jobname": "pubsub-test",
				},
				Entry: push.Entry{
					Timestamp: time.Now(),
					Line:      withTextPayload,
				},
			},
		},
		{
			name: "use-text-payload",
			msg: &pubsub.Message{
				Data: []byte(withTextPayload),
			},
			opts: parseOptions{
				fixedLabels: model.LabelSet{
					"jobname": "pubsub-test",
				},
			},
			expected: loki.Entry{
				Labels: model.LabelSet{
					"jobname": "pubsub-test",
				},
				Entry: push.Entry{
					Timestamp: time.Now(),
					Line:      logTextPayload,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseLogEntry(tt.msg.Data, labels.NewBuilder(labels.EmptyLabels()), tt.relabel, tt.opts)

			require.NoError(t, err)

			require.Equal(t, tt.expected.Labels, got.Labels)
			require.Equal(t, tt.expected.Line, got.Line)
			if tt.opts.useIncomingTimestamp {
				require.Equal(t, tt.expected.Entry.Timestamp, got.Timestamp)
			} else {
				if got.Timestamp.Sub(tt.expected.Timestamp).Seconds() > 1 {
					require.Fail(t, "timestamp shouldn't differ much when rewriting log entry timestamp.")
				}
			}
		})
	}
}

func TestParsePushMessage(t *testing.T) {
	const pushData = "eyJpbnNlcnRJZCI6IjRhZmZhODU4LWU1ZjItNDdmNy05MjU0LWU2MDliNWMwMTRkMCIsImxhYmVscyI6e30sImxvZ05hbWUiOiJwcm9qZWN0cy90ZXN0LXByb2plY3QvbG9ncy9jbG91ZGF1ZGl0Lmdvb2dsZWFwaXMuY29tJTJGZGF0YV9hY2Nlc3MiLCJyZWNlaXZlVGltZXN0YW1wIjoiMjAyMi0wOS0wNlQxODowNzo0My40MTc3MTQwNDZaIiwicmVzb3VyY2UiOnsibGFiZWxzIjp7ImNsdXN0ZXJfbmFtZSI6ImRldi11cy1jZW50cmFsLTQyIiwibG9jYXRpb24iOiJ1cy1jZW50cmFsMSIsInByb2plY3RfaWQiOiJ0ZXN0LXByb2plY3QifSwidHlwZSI6Ims4c19jbHVzdGVyIn0sInRpbWVzdGFtcCI6IjIwMjItMDktMDZUMTg6MDc6NDIuMzYzMTEzWiJ9Cg=="

	type testCase struct {
		name     string
		pm       pushMessageBody
		expected loki.Entry
	}

	tests := []testCase{
		{
			name: "deprecated message id",
			pm: pushMessageBody{
				Message: pushMessage{
					DeprecatedMessageID: "1",
					Data:                pushData,
				},
				Subscription: "test",
			},
			expected: loki.Entry{
				Labels: model.LabelSet{
					"message_id": "1",
				},
			},
		},
		{
			name: "standard message id",
			pm: pushMessageBody{
				Message: pushMessage{
					MessageID: "1",
					Data:      pushData,
				},
				Subscription: "test",
			},
			expected: loki.Entry{
				Labels: model.LabelSet{
					"message_id": "1",
				},
			},
		},
	}

	rc := []*relabel.Config{
		{
			SourceLabels:         model.LabelNames{"__gcp_message_id"},
			Regex:                relabel.MustNewRegexp("(.*)"),
			Action:               relabel.Replace,
			Replacement:          "$1",
			TargetLabel:          "message_id",
			NameValidationScheme: model.LegacyValidation,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry, err := parsePushMessage(tt.pm, rc, "", parseOptions{})
			require.NoError(t, err)
			require.EqualValues(t, tt.expected.Labels, entry.Labels)
		})
	}
}

func TestConvertToLokiCompatibleLabel(t *testing.T) {
	type args struct {
		label string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "Google timestamp label attribute name",
			args: args{
				label: "logging.googleapis.com/timestamp",
			},
			want: "logging_googleapis_com_timestamp",
		},
		{
			name: "Label attribute name with multiple non-underscore characters",
			args: args{
				label: "logging.googleapis.com/Crazy-label",
			},
			want: "logging_googleapis_com_crazy_label",
		},
		{
			name: "Label attribute name in CamelCase converted into SnakeCase",
			args: args{
				label: "logging.googleapis.com/CrazyLabel",
			},
			want: "logging_googleapis_com_crazy_label",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, convertToLokiCompatibleLabel(tt.args.label))
		})
	}
}

func mustTime(t *testing.T, v string) time.Time {
	t.Helper()
	ts, err := time.Parse(time.RFC3339, v)
	require.NoError(t, err)
	return ts
}
