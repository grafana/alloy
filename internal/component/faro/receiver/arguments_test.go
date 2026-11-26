package receiver

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/syntax"
)

func TestSourceMapsArgumentsValidate(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		wantErr string
	}{
		{name: "valid template", path: "/var/build/{{ .Release }}/"},
		{name: "plain path", path: "/var/build/latest/"},
		{name: "unclosed action", path: "/var/build/{{ .Release /", wantErr: "invalid sourcemap location path"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := `
				location {
					path                 = "` + tt.path + `"
					minified_path_prefix = "http://example.com/"
				}
			`
			var args SourceMapsArguments
			err := syntax.Unmarshal([]byte(cfg), &args)
			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.ErrorContains(t, err, tt.wantErr)
		})
	}
}
