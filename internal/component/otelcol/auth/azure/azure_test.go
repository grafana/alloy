package azure_test

import (
	"testing"

	"github.com/grafana/alloy/internal/component/otelcol/auth/azure"
	"github.com/grafana/alloy/syntax"
	"github.com/stretchr/testify/require"
)

func TestUnmarshalAlloy(t *testing.T) {
	tests := []struct {
		name      string
		cfg       string
		expectErr bool
	}{
		{
			name: "use default",
			cfg:  `use_default = true`,
		},
		{
			name: "use default with scopes",
			cfg: `
				use_default = true
				scopes      = ["https://storage.azure.com/.default"]
			`,
		},
		{
			name: "system assigned managed identity",
			cfg:  `managed_identity {}`,
		},
		{
			name: "user assigned managed identity",
			cfg: `
				managed_identity {
					client_id = "11111111-1111-1111-1111-111111111111"
				}
			`,
		},
		{
			name: "workload identity",
			cfg: `
				workload_identity {
					client_id            = "11111111-1111-1111-1111-111111111111"
					tenant_id            = "22222222-2222-2222-2222-222222222222"
					federated_token_file = "/var/run/secrets/azure/tokens/azure-identity-token"
				}
			`,
		},
		{
			name: "empty workload identity",
			cfg: `
				workload_identity {}
			`,
			expectErr: true,
		},
		{
			name: "workload identity missing federated_token_file",
			cfg: `
				workload_identity {
					client_id = "11111111-1111-1111-1111-111111111111"
					tenant_id = "22222222-2222-2222-2222-222222222222"
				}
			`,
			expectErr: true,
		},
		{
			name: "service principal with secret",
			cfg: `
				service_principal {
					tenant_id     = "22222222-2222-2222-2222-222222222222"
					client_id     = "11111111-1111-1111-1111-111111111111"
					client_secret = "supersecret"
				}
			`,
		},
		{
			name: "service principal with certificate",
			cfg: `
				service_principal {
					tenant_id               = "22222222-2222-2222-2222-222222222222"
					client_id               = "11111111-1111-1111-1111-111111111111"
					client_certificate_path = "/etc/azure/cert.pem"
				}
			`,
		},
		{
			name: "empty service principal",
			cfg: `
				workload_identity {}
			`,
			expectErr: true,
		},
		{
			name:      "no authentication method",
			cfg:       ``,
			expectErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var args azure.Arguments
			err := syntax.Unmarshal([]byte(tc.cfg), &args)

			if tc.expectErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}
