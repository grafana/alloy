package alloycli

import (
	"errors"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFormatWrite(t *testing.T) {
	const (
		formattedConfig   = "logging {\n\tlevel = \"debug\"\n}\n"
		unformattedConfig = "logging { level = \"debug\" }"
	)

	writeErr := errors.New("write failed")

	tests := []struct {
		name        string
		input       string
		writeErr    error
		wantWrites  int
		wantContent string
	}{
		{
			name:  "unchanged content",
			input: formattedConfig,
		},
		{
			name:        "changed content",
			input:       unformattedConfig,
			wantWrites:  1,
			wantContent: formattedConfig,
		},
		{
			name:        "write error",
			input:       unformattedConfig,
			writeErr:    writeErr,
			wantWrites:  1,
			wantContent: formattedConfig,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var (
				writes  int
				content []byte
			)
			writer := func(_ string, _ os.FileInfo, r io.Reader) error {
				writes++

				var err error
				content, err = io.ReadAll(r)
				require.NoError(t, err)
				return tc.writeErr
			}

			err := format("test.alloy", nil, strings.NewReader(tc.input), true, false, writer)

			if tc.writeErr != nil {
				require.ErrorIs(t, err, tc.writeErr)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, tc.wantWrites, writes)
			require.Equal(t, tc.wantContent, string(content))
		})
	}
}
