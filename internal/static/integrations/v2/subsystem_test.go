package integrations

import (
	"testing"

	v1 "github.com/grafana/alloy/internal/static/integrations"
	"github.com/grafana/alloy/internal/static/integrations/v2/common"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

func TestSubsystemOptions_Unmarshal(t *testing.T) {
	setRegistered(t, map[Config]Type{
		&testIntegrationA{}: TypeSingleton,
	})

	RegisterLegacy(&legacyConfig{}, TypeSingleton, func(in v1.Config, mc common.MetricsConfig) UpgradedConfig {
		return &legacyShim{Data: in, Common: mc}
	})

	tt := []struct {
		name        string
		in          string
		expectError string
	}{
		{
			name: "invalid integration",
			in: `
        invalidintegration:
          autoscrape:
            enabled: true
      `,
			expectError: "line 2: field invalidintegration not found in type integrations.SubsystemOptions",
		},
		{
			name: "invalid field",
			in: `
        test:
          invalidfield: true
      `,
			expectError: "line 1: field invalidfield not found in type integrations.plain",
		},
		{
			name: "invalid v1 field",
			in: `
        legacy:
          invalidfield: true
      `,
			expectError: "line 1: field invalidfield not found",
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			var so SubsystemOptions
			err := yaml.UnmarshalStrict([]byte(tc.in), &so)

			var te *yaml.TypeError
			require.ErrorAs(t, err, &te)
			require.Len(t, te.Errors, 1)
			require.Equal(t, tc.expectError, te.Errors[0])
		})
	}
}
