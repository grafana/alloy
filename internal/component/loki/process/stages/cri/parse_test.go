package cri

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParse(t *testing.T) {
	type testCase struct {
		name       string
		line       string
		wantTS     string
		wantValid  bool
		wantStream Stream
		wantFlag   Flag
		wantBody   string
	}

	tests := []testCase{
		{
			name:       "partial stdout",
			line:       "2019-01-01T01:00:00.000000001Z stdout P my super cool message",
			wantTS:     "2019-01-01T01:00:00.000000001Z",
			wantValid:  true,
			wantStream: StreamStdOut,
			wantFlag:   FlagPartial,
			wantBody:   "my super cool message",
		},
		{
			name:       "full stdout",
			line:       "2019-01-01T01:00:00.000000001Z stdout F my super cool message",
			wantTS:     "2019-01-01T01:00:00.000000001Z",
			wantValid:  true,
			wantStream: StreamStdOut,
			wantFlag:   FlagFull,
			wantBody:   "my super cool message",
		},
		{
			name:       "missing flag defaults to full",
			line:       "2019-01-01T01:00:00.000000001Z stdout my super cool message",
			wantTS:     "2019-01-01T01:00:00.000000001Z",
			wantValid:  true,
			wantStream: StreamStdOut,
			wantFlag:   FlagFull,
			wantBody:   "my super cool message",
		},
		{
			name:      "unknown stream is invalid",
			wantValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, ok := ParseCRI(tt.line)
			require.Equal(t, tt.wantValid, ok, "valid")
			require.Equal(t, tt.wantTS, got.Timestamp, "timestamp")
			require.Equal(t, tt.wantStream, got.Stream, "stream")
			require.Equal(t, tt.wantFlag, got.Flag, "flag")
			require.Equal(t, tt.wantBody, got.Content, "content")
		})
	}
}
