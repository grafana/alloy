package gcplogtarget

import (
	"testing"
	"time"

	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/relabel"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component/common/loki"
)

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
