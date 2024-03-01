package host_info

import (
	"testing"
	"time"

	"gotest.tools/assert"
)

func TestValidate(t *testing.T) {
	tests := []struct {
		name string
		cfg  *Config
		err  string
	}{
		{
			name: "valid config",
			cfg: &Config{
				HostIdentifiers:      []string{"host.id"},
				MetricsFlushInterval: 1 * time.Minute,
			},
		},
		{
			name: "invalid host identifiers",
			cfg: &Config{
				HostIdentifiers: nil,
			},
			err: "at least one host identifier is required",
		},
		{
			name: "invalid metrics flush interval",
			cfg: &Config{
				HostIdentifiers:      []string{"host.id"},
				MetricsFlushInterval: 1 * time.Second,
			},
			err: "\"1s\" is not a valid flush interval",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.cfg.Validate()
			if tc.err != "" {
				assert.Error(t, err, tc.err)
			} else {
				assert.NilError(t, err)
			}
		})
	}
}
