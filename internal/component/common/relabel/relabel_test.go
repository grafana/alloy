package relabel

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/grafana/river"
)

func TestParseConfig(t *testing.T) {
	for _, tt := range []struct {
		name      string
		cfg       string
		expectErr bool
	}{
		{
			name: "valid keepequal config",
			cfg: `
			action = "keepequal"
			target_label = "foo"
			source_labels = ["bar"]
			`,
		},
		{
			name: "valid dropequal config",
			cfg: `
			action = "dropequal"
			target_label = "foo"
			source_labels = ["bar"]
			`,
		},
		{
			name: "missing dropequal target",
			cfg: `
			action = "dropequal"
			source_labels = ["bar"]
			`,
			expectErr: true,
		},
		{
			name: "missing keepequal target",
			cfg: `
			action = "keepequal"
			source_labels = ["bar"]
			`,
			expectErr: true,
		},
		{
			name: "unknown action",
			cfg: `
			action = "foo"
			`,
			expectErr: true,
		},
	} {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var cfg Config
			err := river.Unmarshal([]byte(tt.cfg), &cfg)
			if tt.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
