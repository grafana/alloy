package gcplogtarget

import (
	"testing"

	"github.com/grafana/regexp"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/relabel"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component/common/loki"
)

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

const pushData = "eyJpbnNlcnRJZCI6IjRhZmZhODU4LWU1ZjItNDdmNy05MjU0LWU2MDliNWMwMTRkMCIsImxhYmVscyI6e30sImxvZ05hbWUiOiJwcm9qZWN0cy90ZXN0LXByb2plY3QvbG9ncy9jbG91ZGF1ZGl0Lmdvb2dsZWFwaXMuY29tJTJGZGF0YV9hY2Nlc3MiLCJyZWNlaXZlVGltZXN0YW1wIjoiMjAyMi0wOS0wNlQxODowNzo0My40MTc3MTQwNDZaIiwicmVzb3VyY2UiOnsibGFiZWxzIjp7ImNsdXN0ZXJfbmFtZSI6ImRldi11cy1jZW50cmFsLTQyIiwibG9jYXRpb24iOiJ1cy1jZW50cmFsMSIsInByb2plY3RfaWQiOiJ0ZXN0LXByb2plY3QifSwidHlwZSI6Ims4c19jbHVzdGVyIn0sInRpbWVzdGFtcCI6IjIwMjItMDktMDZUMTg6MDc6NDIuMzYzMTEzWiJ9Cg=="

func TestTranslate(t *testing.T) {
	type testCase struct {
		name     string
		pm       PushMessageBody
		expected loki.Entry
	}

	tests := []testCase{
		{
			name: "deprecated message id",
			pm: PushMessageBody{
				Message: PushMessage{
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
			pm: PushMessageBody{
				Message: PushMessage{
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
			SourceLabels: model.LabelNames{"__gcp_message_id"},
			Regex:        mustNewRegexp("(.*)"),
			Action:       relabel.Replace,
			Replacement:  "$1",
			TargetLabel:  "message_id",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry, err := translate(tt.pm, model.LabelSet{}, false, false, rc, "")
			require.NoError(t, err)
			require.EqualValues(t, tt.expected.Labels, entry.Labels)
		})
	}
}

func mustNewRegexp(s string) relabel.Regexp {
	re, err := regexp.Compile("^(?:" + s + ")$")
	if err != nil {
		panic(err)
	}
	return relabel.Regexp{Regexp: re}
}
